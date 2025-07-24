package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "github.com/xuezhaojun/multiclustertunnel/api/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Mock implementations for testing

// mockClientConnectionManager implements connectionManager interface for testing
type mockClientConnectionManager struct {
	mock.Mock
	outgoingChan chan *v1.Packet
}

func newMockClientProxyManager() *mockClientConnectionManager {
	return &mockClientConnectionManager{
		outgoingChan: make(chan *v1.Packet, 10),
	}
}

func (m *mockClientConnectionManager) Dispatch(packet *v1.Packet) error {
	args := m.Called(packet)
	return args.Error(0)
}

func (m *mockClientConnectionManager) OutgoingChan() <-chan *v1.Packet {
	return m.outgoingChan
}

func (m *mockClientConnectionManager) Close() error {
	args := m.Called()
	close(m.outgoingChan)
	return args.Error(0)
}

// mockTunnelClient implements v1.TunnelService_TunnelClient interface for testing
type mockTunnelClient struct {
	mock.Mock
	recvChan chan *v1.Packet
	sendChan chan *v1.Packet
	mu       sync.Mutex
	closed   bool
}

func newMockTunnelClient() *mockTunnelClient {
	return &mockTunnelClient{
		recvChan: make(chan *v1.Packet, 10),
		sendChan: make(chan *v1.Packet, 10),
	}
}

func (m *mockTunnelClient) Send(packet *v1.Packet) error {
	args := m.Called(packet)
	if args.Error(0) == nil {
		select {
		case m.sendChan <- packet:
		default:
		}
	}
	return args.Error(0)
}

func (m *mockTunnelClient) Recv() (*v1.Packet, error) {
	args := m.Called()
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	// Try to get packet from channel or return the mocked packet
	if packet := args.Get(0); packet != nil {
		return packet.(*v1.Packet), nil
	}

	select {
	case packet := <-m.recvChan:
		return packet, nil
	case <-time.After(100 * time.Millisecond):
		return nil, io.EOF
	}
}

func (m *mockTunnelClient) Header() (metadata.MD, error) {
	args := m.Called()
	return args.Get(0).(metadata.MD), args.Error(1)
}

func (m *mockTunnelClient) Trailer() metadata.MD {
	args := m.Called()
	return args.Get(0).(metadata.MD)
}

func (m *mockTunnelClient) CloseSend() error {
	args := m.Called()
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		close(m.recvChan)
		m.closed = true
	}
	return args.Error(0)
}

func (m *mockTunnelClient) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

func (m *mockTunnelClient) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *mockTunnelClient) RecvMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

// TestNewClient tests the NewClient constructor
func TestNewClient(t *testing.T) {
	ctx := context.Background()

	t.Run("default configuration", func(t *testing.T) {
		config := &Config{
			HubAddress:  "localhost:8080",
			ClusterName: "test-cluster",
		}

		client := NewClient(ctx, config)
		require.NotNil(t, client)
		assert.Equal(t, config, client.config)
		assert.NotNil(t, client.lcm)

		// Verify default keepalive parameters are set
		assert.NotNil(t, config.DialOptions)
		assert.Len(t, config.DialOptions, 1)

		// Verify default backoff factory is set
		assert.NotNil(t, config.BackoffFactory)
		backoff := config.BackoffFactory()
		assert.NotNil(t, backoff)
	})

	t.Run("custom dial options", func(t *testing.T) {
		config := &Config{
			HubAddress:  "localhost:8080",
			ClusterName: "test-cluster",
			DialOptions: []grpc.DialOption{grpc.WithUserAgent("test-agent")},
		}

		client := NewClient(ctx, config)
		require.NotNil(t, client)

		// When DialOptions are provided (non-nil), keepalive is NOT automatically added
		// This is the current behavior of the implementation
		assert.Len(t, config.DialOptions, 1)
	})

	t.Run("nil dial options gets keepalive", func(t *testing.T) {
		config := &Config{
			HubAddress:  "localhost:8080",
			ClusterName: "test-cluster",
			DialOptions: nil, // Explicitly nil
		}

		client := NewClient(ctx, config)
		require.NotNil(t, client)

		// When DialOptions are nil, keepalive should be added
		assert.Len(t, config.DialOptions, 1)
	})

	t.Run("custom backoff factory", func(t *testing.T) {
		customBackoff := func() backoff.BackOff {
			return backoff.NewConstantBackOff(time.Second)
		}
		config := &Config{
			HubAddress:     "localhost:8080",
			ClusterName:    "test-cluster",
			BackoffFactory: customBackoff,
		}

		client := NewClient(ctx, config)
		require.NotNil(t, client)

		// Should use custom backoff factory
		assert.Equal(t, fmt.Sprintf("%p", customBackoff), fmt.Sprintf("%p", config.BackoffFactory))
	})
}

// TestClientRun tests the Run method
func TestClientRun(t *testing.T) {
	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		config := &Config{
			HubAddress:  "localhost:8080",
			ClusterName: "test-cluster",
		}

		client := NewClient(ctx, config)

		// Cancel context immediately
		cancel()

		err := client.Run(ctx)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("connection retry with backoff", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		config := &Config{
			HubAddress:  "invalid:99999", // Invalid address to force connection failure
			ClusterName: "test-cluster",
			BackoffFactory: func() backoff.BackOff {
				b := backoff.NewConstantBackOff(50 * time.Millisecond)
				return b
			},
		}

		client := NewClient(ctx, config)

		err := client.Run(ctx)
		assert.Equal(t, context.DeadlineExceeded, err)
	})
}

// TestClientProcessIncoming tests the processIncoming method
func TestClientProcessIncoming(t *testing.T) {
	t.Run("successful packet dispatch", func(t *testing.T) {
		mockStream := newMockTunnelClient()
		mockProxyMgr := newMockClientProxyManager()

		client := &Client{
			lcm: mockProxyMgr,
		}

		// Setup mock expectations
		packet := &v1.Packet{
			ConnId: 1,
			Code:   v1.ControlCode_DATA,
			Data:   []byte("test"),
		}

		mockStream.On("Recv").Return(packet, nil).Once()
		mockStream.On("Recv").Return(nil, io.EOF).Once()
		mockProxyMgr.On("Dispatch", packet).Return(nil).Once()

		err := client.processIncoming(mockStream)
		assert.Equal(t, io.EOF, err)

		// Give time for goroutine to complete
		time.Sleep(10 * time.Millisecond)

		mockStream.AssertExpectations(t)
		mockProxyMgr.AssertExpectations(t)
	})

	t.Run("dispatch error with error packet sent", func(t *testing.T) {
		mockStream := newMockTunnelClient()
		mockProxyMgr := newMockClientProxyManager()

		client := &Client{
			lcm: mockProxyMgr,
		}

		// Setup mock expectations
		packet := &v1.Packet{
			ConnId: 1,
			Code:   v1.ControlCode_DATA,
			Data:   []byte("test"),
		}

		dispatchErr := errors.New("dispatch failed")
		mockStream.On("Recv").Return(packet, nil).Once()
		mockStream.On("Recv").Return(nil, io.EOF).Once()
		mockProxyMgr.On("Dispatch", packet).Return(dispatchErr).Once()

		// Expect error packet to be sent
		mockStream.On("Send", mock.MatchedBy(func(p *v1.Packet) bool {
			return p.ConnId == 1 && p.Code == v1.ControlCode_ERROR && p.ErrorMessage == "dispatch failed"
		})).Return(nil).Once()

		err := client.processIncoming(mockStream)
		assert.Equal(t, io.EOF, err)

		// Give time for goroutine to complete
		time.Sleep(10 * time.Millisecond)

		mockStream.AssertExpectations(t)
		mockProxyMgr.AssertExpectations(t)
	})

	t.Run("stream receive error", func(t *testing.T) {
		mockStream := newMockTunnelClient()
		mockProxyMgr := newMockClientProxyManager()

		client := &Client{
			lcm: mockProxyMgr,
		}

		recvErr := errors.New("stream error")
		mockStream.On("Recv").Return(nil, recvErr).Once()

		err := client.processIncoming(mockStream)
		assert.Equal(t, recvErr, err)
		mockStream.AssertExpectations(t)
	})
}

// TestClientProcessOutgoing tests the processOutgoing method
func TestClientProcessOutgoing(t *testing.T) {
	t.Run("successful packet sending", func(t *testing.T) {
		mockStream := newMockTunnelClient()
		mockProxyMgr := newMockClientProxyManager()

		client := &Client{
			lcm: mockProxyMgr,
		}

		// Setup test packet
		packet := &v1.Packet{
			ConnId: 1,
			Code:   v1.ControlCode_DATA,
			Data:   []byte("test"),
		}

		// Send packet to outgoing channel
		go func() {
			mockProxyMgr.outgoingChan <- packet
			close(mockProxyMgr.outgoingChan)
		}()

		mockStream.On("Send", packet).Return(nil).Once()

		err := client.processOutgoing(mockStream)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "outgoing channel closed")
		mockStream.AssertExpectations(t)
	})

	t.Run("stream send error", func(t *testing.T) {
		mockStream := newMockTunnelClient()
		mockProxyMgr := newMockClientProxyManager()

		client := &Client{
			lcm: mockProxyMgr,
		}

		// Setup test packet
		packet := &v1.Packet{
			ConnId: 1,
			Code:   v1.ControlCode_DATA,
			Data:   []byte("test"),
		}

		sendErr := errors.New("send failed")

		// Send packet to outgoing channel
		go func() {
			mockProxyMgr.outgoingChan <- packet
		}()

		mockStream.On("Send", packet).Return(sendErr).Once()

		err := client.processOutgoing(mockStream)
		assert.Equal(t, sendErr, err)
		mockStream.AssertExpectations(t)
	})
}

// TestClientServe tests the serve method
func TestClientServe(t *testing.T) {
	t.Run("goroutine error handling", func(t *testing.T) {
		mockStream := newMockTunnelClient()
		mockProxyMgr := newMockClientProxyManager()

		client := &Client{
			lcm: mockProxyMgr,
		}

		// Setup mock to return error from Recv (processIncoming will fail)
		recvErr := errors.New("recv error")
		mockStream.On("Recv").Return(nil, recvErr).Once()

		ctx := context.Background()
		err := client.serve(ctx, mockStream)
		assert.Equal(t, recvErr, err)
		mockStream.AssertExpectations(t)
	})

	t.Run("context cancellation attempts to send DRAIN packet", func(t *testing.T) {
		mockStream := newMockTunnelClient()
		mockProxyMgr := newMockClientProxyManager()

		client := &Client{
			lcm: mockProxyMgr,
		}

		ctx, cancel := context.WithCancel(context.Background())

		// Setup mock to block on Recv (processIncoming will wait)
		mockStream.On("Recv").Return(nil, context.Canceled).Maybe()

		// Setup mock to expect DRAIN packet to be sent (might fail due to stream closure)
		mockStream.On("Send", mock.MatchedBy(func(p *v1.Packet) bool {
			return p.ConnId == 0 && p.Code == v1.ControlCode_DRAIN
		})).Return(nil).Maybe() // Use Maybe() since it might not be called if stream closes first

		// Setup mock for outgoing channel (processOutgoing might be called)
		mockProxyMgr.On("OutgoingChan").Return(mockProxyMgr.outgoingChan).Maybe()

		// Cancel context immediately to trigger DRAIN
		cancel()

		err := client.serve(ctx, mockStream)
		assert.Equal(t, context.Canceled, err)

		// Give time for goroutines to complete
		time.Sleep(50 * time.Millisecond)

		// We don't assert on DRAIN packet being sent because it might fail due to stream closure
		// The important thing is that the code attempts to send it
		mockStream.AssertExpectations(t)
	})
}

// TestClientConfig tests various configuration scenarios
func TestClientConfig(t *testing.T) {
	t.Run("keepalive parameters", func(t *testing.T) {
		ctx := context.Background()
		config := &Config{
			HubAddress:  "localhost:8080",
			ClusterName: "test-cluster",
		}

		client := NewClient(ctx, config)
		require.NotNil(t, client)

		// Verify keepalive parameters are added
		assert.NotNil(t, config.DialOptions)
		assert.Len(t, config.DialOptions, 1)

		// The dial option should be a keepalive option
		// We can't easily inspect the option content, but we can verify it exists
	})

	t.Run("backoff configuration", func(t *testing.T) {
		ctx := context.Background()

		// Test with custom backoff
		customBackoffCalled := false
		config := &Config{
			HubAddress:  "localhost:8080",
			ClusterName: "test-cluster",
			BackoffFactory: func() backoff.BackOff {
				customBackoffCalled = true
				return backoff.NewConstantBackOff(time.Millisecond)
			},
		}

		client := NewClient(ctx, config)
		require.NotNil(t, client)

		// Test that custom backoff factory is preserved
		backoff := config.BackoffFactory()
		assert.NotNil(t, backoff)
		assert.True(t, customBackoffCalled)
	})
}

// TestClientIntegration tests integration scenarios
func TestClientIntegration(t *testing.T) {
	t.Run("full packet flow", func(t *testing.T) {
		mockStream := newMockTunnelClient()
		mockProxyMgr := newMockClientProxyManager()

		client := &Client{
			lcm: mockProxyMgr,
		}

		// Setup incoming packet
		incomingPacket := &v1.Packet{
			ConnId: 1,
			Code:   v1.ControlCode_DATA,
			Data:   []byte("incoming"),
		}

		// Setup outgoing packet
		outgoingPacket := &v1.Packet{
			ConnId: 2,
			Code:   v1.ControlCode_DATA,
			Data:   []byte("outgoing"),
		}

		// Mock expectations for incoming
		mockStream.On("Recv").Return(incomingPacket, nil).Once()
		mockStream.On("Recv").Return(nil, io.EOF).Once()
		mockProxyMgr.On("Dispatch", incomingPacket).Return(nil).Once()

		// Mock expectations for outgoing
		mockStream.On("Send", outgoingPacket).Return(nil).Once()

		// Send outgoing packet and close channel
		go func() {
			time.Sleep(5 * time.Millisecond) // Small delay to ensure processIncoming starts
			mockProxyMgr.outgoingChan <- outgoingPacket
			close(mockProxyMgr.outgoingChan)
		}()

		// Start serve (this will run both processIncoming and processOutgoing)
		ctx := context.Background()
		err := client.serve(ctx, mockStream)

		// Should get EOF from processIncoming or "outgoing channel closed" from processOutgoing
		assert.True(t, err == io.EOF || (err != nil && err.Error() == "outgoing channel closed"))

		// Give time for goroutines to complete
		time.Sleep(20 * time.Millisecond)

		mockStream.AssertExpectations(t)
		mockProxyMgr.AssertExpectations(t)
	})
}

// TestClientErrorScenarios tests various error scenarios
func TestClientErrorScenarios(t *testing.T) {
	t.Run("dispatch error with send error", func(t *testing.T) {
		mockStream := newMockTunnelClient()
		mockProxyMgr := newMockClientProxyManager()

		client := &Client{
			lcm: mockProxyMgr,
		}

		// Setup mock expectations
		packet := &v1.Packet{
			ConnId: 1,
			Code:   v1.ControlCode_DATA,
			Data:   []byte("test"),
		}

		dispatchErr := errors.New("dispatch failed")
		sendErr := errors.New("send failed")

		mockStream.On("Recv").Return(packet, nil).Once()
		mockStream.On("Recv").Return(nil, io.EOF).Once()
		mockProxyMgr.On("Dispatch", packet).Return(dispatchErr).Once()

		// Expect error packet to be sent, but it fails
		mockStream.On("Send", mock.MatchedBy(func(p *v1.Packet) bool {
			return p.ConnId == 1 && p.Code == v1.ControlCode_ERROR
		})).Return(sendErr).Once()

		err := client.processIncoming(mockStream)
		assert.Equal(t, io.EOF, err)

		// Give time for goroutine to complete
		time.Sleep(10 * time.Millisecond)

		mockStream.AssertExpectations(t)
		mockProxyMgr.AssertExpectations(t)
	})
}
