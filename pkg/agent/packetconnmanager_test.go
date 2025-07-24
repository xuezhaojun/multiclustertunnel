package agent

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/xuezhaojun/multiclustertunnel/api/v1"
)

// mockServer creates a simple TCP server for testing
type mockServer struct {
	listener net.Listener
	addr     string
	data     []byte
	mu       sync.Mutex
	closed   bool
}

func newMockServer() (*mockServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	server := &mockServer{
		listener: listener,
		addr:     listener.Addr().String(),
	}

	go server.serve()
	return server, nil
}

func (s *mockServer) serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		go func(c net.Conn) {
			defer c.Close()

			// Echo server: read data and write it back
			buffer := make([]byte, 1024)
			for {
				n, err := c.Read(buffer)
				if err != nil {
					return
				}

				// Store received data for verification
				s.mu.Lock()
				s.data = append(s.data, buffer[:n]...)
				s.mu.Unlock()

				// Echo back
				_, err = c.Write(buffer[:n])
				if err != nil {
					return
				}
			}
		}(conn)
	}
}

func (s *mockServer) getReceivedData() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.data...)
}

func (s *mockServer) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.listener.Close()
		s.closed = true
	}
}

func TestDefaultConnectionManagerConfig(t *testing.T) {
	config := DefaultPacketConnManagerConfig()

	assert.Equal(t, connReadBufferSize, config.ReadBufferSize)
	assert.Equal(t, outgoingChanSize, config.OutgoingChanSize)
	assert.Equal(t, dialTimeout, config.DialTimeout)
}

func TestNewConnectionManager(t *testing.T) {
	ctx := context.Background()

	cm := newPacketConnectionManager(ctx)
	require.NotNil(t, cm)

	// Test that outgoing channel is available
	outgoing := cm.OutgoingChan()
	require.NotNil(t, outgoing)

	// Clean up
	err := cm.Close()
	assert.NoError(t, err)
}

func TestNewConnectionManagerWithConfig(t *testing.T) {
	ctx := context.Background()

	config := &PacketConnManagerConfig{
		ReadBufferSize:   16 * 1024,
		OutgoingChanSize: 50,
		DialTimeout:      5 * time.Second,
	}

	cm := newPacketConnectionManagerWithConfig(ctx, config)
	require.NotNil(t, cm)

	// Verify config is used
	impl := cm.(*packetConnManagerImpl)
	assert.Equal(t, config, impl.config)

	// Clean up
	err := cm.Close()
	assert.NoError(t, err)
}

func TestDispatchDataPacket_NewConnection(t *testing.T) {
	// Start mock server
	server, err := newMockServer()
	require.NoError(t, err)
	defer server.close()

	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	// Create a data packet
	packet := &v1.Packet{
		ConnId:        1,
		Code:          v1.ControlCode_DATA,
		Data:          []byte("hello world"),
		TargetAddress: server.addr,
	}

	// Dispatch the packet
	err = cm.Dispatch(packet)
	assert.NoError(t, err)

	// Wait a bit for connection to be established and data to be sent
	time.Sleep(100 * time.Millisecond)

	// Verify data was received by mock server
	receivedData := server.getReceivedData()
	assert.Equal(t, []byte("hello world"), receivedData)
}

func TestDispatchDataPacket_ExistingConnection(t *testing.T) {
	// Start mock server
	server, err := newMockServer()
	require.NoError(t, err)
	defer server.close()

	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	// Create first packet to establish connection
	packet1 := &v1.Packet{
		ConnId:        1,
		Code:          v1.ControlCode_DATA,
		Data:          []byte("first"),
		TargetAddress: server.addr,
	}

	err = cm.Dispatch(packet1)
	assert.NoError(t, err)

	// Wait for connection to be established
	time.Sleep(50 * time.Millisecond)

	// Send second packet on same connection
	packet2 := &v1.Packet{
		ConnId:        1,
		Code:          v1.ControlCode_DATA,
		Data:          []byte("second"),
		TargetAddress: server.addr, // Should be ignored for existing connection
	}

	err = cm.Dispatch(packet2)
	assert.NoError(t, err)

	// Wait for data to be sent
	time.Sleep(100 * time.Millisecond)

	// Verify both packets were received
	receivedData := server.getReceivedData()
	assert.Equal(t, []byte("firstsecond"), receivedData)
}

func TestDispatchErrorPacket(t *testing.T) {
	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	// Create an error packet
	packet := &v1.Packet{
		ConnId:       1,
		Code:         v1.ControlCode_ERROR,
		ErrorMessage: "test error",
	}

	// Dispatch the packet
	err := cm.Dispatch(packet)
	assert.NoError(t, err)

	// Verify connection is not created for error packets
	impl := cm.(*packetConnManagerImpl)
	impl.connLock.RLock()
	assert.Empty(t, impl.localConnections)
	impl.connLock.RUnlock()
}

func TestDispatchUnknownControlCode(t *testing.T) {
	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	// Create packet with unknown control code
	packet := &v1.Packet{
		ConnId: 1,
		Code:   v1.ControlCode(999), // Invalid code
		Data:   []byte("test"),
	}

	// Dispatch should return error
	err := cm.Dispatch(packet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown control code")
}

func TestMissingTargetAddress(t *testing.T) {
	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	packet := &v1.Packet{
		ConnId:        1,
		Code:          v1.ControlCode_DATA,
		Data:          []byte("test"),
		TargetAddress: "", // Missing target address
	}

	err := cm.Dispatch(packet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to dial target")
}

func TestConnectionDialError(t *testing.T) {
	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	packet := &v1.Packet{
		ConnId:        1,
		Code:          v1.ControlCode_DATA,
		Data:          []byte("test"),
		TargetAddress: "127.0.0.1:99999", // Invalid port
	}

	err := cm.Dispatch(packet)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to dial target")
}

func TestOutgoingDataFlow(t *testing.T) {
	// Start mock server
	server, err := newMockServer()
	require.NoError(t, err)
	defer server.close()

	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	// Create a data packet to establish connection
	packet := &v1.Packet{
		ConnId:        1,
		Code:          v1.ControlCode_DATA,
		Data:          []byte("test"),
		TargetAddress: server.addr,
	}

	err = cm.Dispatch(packet)
	require.NoError(t, err)

	// Wait for connection and echo response
	time.Sleep(100 * time.Millisecond)

	// Check outgoing channel for echoed data
	outgoing := cm.OutgoingChan()
	select {
	case outPacket := <-outgoing:
		assert.Equal(t, int64(1), outPacket.ConnId)
		assert.Equal(t, v1.ControlCode_DATA, outPacket.Code)
		assert.Equal(t, []byte("test"), outPacket.Data)
	case <-time.After(1 * time.Second):
		t.Fatal("Expected outgoing packet but got timeout")
	}
}

func TestMultipleConnections(t *testing.T) {
	// Start mock server
	server, err := newMockServer()
	require.NoError(t, err)
	defer server.close()

	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	// Create packets for different connections
	packet1 := &v1.Packet{
		ConnId:        1,
		Code:          v1.ControlCode_DATA,
		Data:          []byte("conn1"),
		TargetAddress: server.addr,
	}

	packet2 := &v1.Packet{
		ConnId:        2,
		Code:          v1.ControlCode_DATA,
		Data:          []byte("conn2"),
		TargetAddress: server.addr,
	}

	// Dispatch both packets
	err = cm.Dispatch(packet1)
	require.NoError(t, err)

	err = cm.Dispatch(packet2)
	require.NoError(t, err)

	// Wait for connections to be established
	time.Sleep(100 * time.Millisecond)

	// Verify both connections exist
	impl := cm.(*packetConnManagerImpl)
	impl.connLock.RLock()
	assert.Len(t, impl.localConnections, 2)
	assert.Contains(t, impl.localConnections, int64(1))
	assert.Contains(t, impl.localConnections, int64(2))
	impl.connLock.RUnlock()
}

func TestConnectionCleanupOnError(t *testing.T) {
	// Start mock server
	server, err := newMockServer()
	require.NoError(t, err)

	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	// Create a data packet to establish connection
	packet := &v1.Packet{
		ConnId:        1,
		Code:          v1.ControlCode_DATA,
		Data:          []byte("test"),
		TargetAddress: server.addr,
	}

	err = cm.Dispatch(packet)
	require.NoError(t, err)

	// Wait for connection to be established
	time.Sleep(50 * time.Millisecond)

	// Verify connection exists
	impl := cm.(*packetConnManagerImpl)
	impl.connLock.RLock()
	assert.Len(t, impl.localConnections, 1)
	impl.connLock.RUnlock()

	// Send an ERROR packet to trigger cleanup
	errorPacket := &v1.Packet{
		ConnId:       1,
		Code:         v1.ControlCode_ERROR,
		ErrorMessage: "simulated error",
	}

	err = cm.Dispatch(errorPacket)
	require.NoError(t, err)

	// Wait a bit for cleanup
	time.Sleep(100 * time.Millisecond)

	// Verify connection was cleaned up
	impl.connLock.RLock()
	assert.Empty(t, impl.localConnections)
	impl.connLock.RUnlock()

	// Close the server
	server.close()
}

func TestProxyManagerClose(t *testing.T) {
	// Start mock server
	server, err := newMockServer()
	require.NoError(t, err)
	defer server.close()

	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)

	// Create connections
	packet1 := &v1.Packet{ConnId: 1, Code: v1.ControlCode_DATA, Data: []byte("test1"), TargetAddress: server.addr}
	packet2 := &v1.Packet{ConnId: 2, Code: v1.ControlCode_DATA, Data: []byte("test2"), TargetAddress: server.addr}

	err = cm.Dispatch(packet1)
	require.NoError(t, err)
	err = cm.Dispatch(packet2)
	require.NoError(t, err)

	// Wait for connections
	time.Sleep(50 * time.Millisecond)

	// Verify connections exist
	impl := cm.(*packetConnManagerImpl)
	impl.connLock.RLock()
	assert.Len(t, impl.localConnections, 2)
	impl.connLock.RUnlock()

	// Close proxy manager
	err = cm.Close()
	assert.NoError(t, err)

	// Verify all connections are cleaned up
	impl.connLock.RLock()
	assert.Empty(t, impl.localConnections)
	impl.connLock.RUnlock()

	// Verify outgoing channel is closed
	// We need to drain any remaining packets first
	outgoing := cm.OutgoingChan()
	for {
		select {
		case _, ok := <-outgoing:
			if !ok {
				// Channel is closed, this is what we expect
				return
			}
			// Continue draining
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Expected closed channel but got timeout")
		}
	}
}

func TestConcurrentDispatch(t *testing.T) {
	// Start mock server
	server, err := newMockServer()
	require.NoError(t, err)
	defer server.close()

	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	// Dispatch packets concurrently
	const numGoroutines = 10
	const packetsPerGoroutine = 5

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < packetsPerGoroutine; j++ {
				packet := &v1.Packet{
					ConnId:        int64(goroutineID*packetsPerGoroutine + j + 1),
					Code:          v1.ControlCode_DATA,
					Data:          []byte(fmt.Sprintf("data-%d-%d", goroutineID, j)),
					TargetAddress: server.addr,
				}
				err := cm.Dispatch(packet)
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Wait for all connections to be established
	time.Sleep(200 * time.Millisecond)

	// Verify all connections were created
	impl := cm.(*packetConnManagerImpl)
	impl.connLock.RLock()
	expectedConnections := numGoroutines * packetsPerGoroutine
	assert.Len(t, impl.localConnections, expectedConnections)
	impl.connLock.RUnlock()
}

func TestCustomConfig(t *testing.T) {
	ctx := context.Background()

	config := &PacketConnManagerConfig{
		ReadBufferSize:   8 * 1024, // 8KB
		OutgoingChanSize: 25,       // Smaller buffer
		IncomingChanSize: 10,       // Smaller incoming buffer
		DialTimeout:      1 * time.Second,
	}

	cm := newPacketConnectionManagerWithConfig(ctx, config)
	defer cm.Close()

	impl := cm.(*packetConnManagerImpl)
	assert.Equal(t, 8*1024, impl.config.ReadBufferSize)
	assert.Equal(t, 25, impl.config.OutgoingChanSize)
	assert.Equal(t, 10, impl.config.IncomingChanSize)
	assert.Equal(t, 1*time.Second, impl.config.DialTimeout)
}

// TestPacketOrderingForSameConnection tests that packets with the same conn_id
// are processed sequentially to maintain TCP ordering semantics
func TestPacketOrderingForSameConnection(t *testing.T) {
	// Start mock server that records received data in order
	server, err := newMockServer()
	require.NoError(t, err)
	defer server.close()

	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	connID := int64(1)
	numPackets := 10
	packetSize := 100

	// Send multiple packets with the same conn_id rapidly
	for i := 0; i < numPackets; i++ {
		// Create packet with sequential data
		data := make([]byte, packetSize)
		for j := 0; j < packetSize; j++ {
			data[j] = byte(i) // Each packet has data filled with its sequence number
		}

		packet := &v1.Packet{
			ConnId:        connID,
			Code:          v1.ControlCode_DATA,
			Data:          data,
			TargetAddress: server.addr,
		}

		err := cm.Dispatch(packet)
		assert.NoError(t, err)
	}

	// Wait for all packets to be processed
	time.Sleep(200 * time.Millisecond)

	// Verify that data was received in the correct order
	receivedData := server.getReceivedData()
	expectedLength := numPackets * packetSize
	assert.Equal(t, expectedLength, len(receivedData), "Should receive all data")

	// Verify the order - each 100-byte chunk should have the same byte value
	for i := 0; i < numPackets; i++ {
		start := i * packetSize
		end := start + packetSize
		chunk := receivedData[start:end]

		// All bytes in this chunk should be the same (the packet sequence number)
		expectedByte := byte(i)
		for j, b := range chunk {
			assert.Equal(t, expectedByte, b,
				"Packet %d, byte %d should be %d but got %d - packets may be out of order",
				i, j, expectedByte, b)
		}
	}
}
