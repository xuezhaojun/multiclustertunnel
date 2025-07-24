package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/xuezhaojun/multiclustertunnel/api/v1"
)

// TestConcurrentConnectionLimits tests the maximum number of concurrent connections
// This is a performance test that creates many connections and should be run separately
func TestConcurrentConnectionLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Start mock server
	server, err := newMockServer()
	require.NoError(t, err)
	defer server.close()

	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	// Test different connection counts to find the limit
	testCases := []struct {
		name        string
		connections int
		expectError bool
	}{
		{"100 connections", 100, false},
		{"500 connections", 500, false},
		{"1000 connections", 1000, false}, // System can handle this many
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fresh proxy manager for each test
			cmTest := newPacketConnectionManager(ctx)
			defer cmTest.Close()

			successCount := 0
			errorCount := 0

			// Create connections concurrently
			var wg sync.WaitGroup
			var mu sync.Mutex

			for i := 0; i < tc.connections; i++ {
				wg.Add(1)
				go func(connID int) {
					defer wg.Done()

					packet := &v1.Packet{
						ConnId:        int64(connID + 1),
						Code:          v1.ControlCode_DATA,
						Data:          []byte(fmt.Sprintf("data-%d", connID)),
						TargetAddress: server.addr,
					}

					err := cmTest.Dispatch(packet)

					mu.Lock()
					if err != nil {
						errorCount++
					} else {
						successCount++
					}
					mu.Unlock()
				}(i)
			}

			wg.Wait()

			// Wait for connections to be established
			time.Sleep(500 * time.Millisecond)

			t.Logf("Test: %s", tc.name)
			t.Logf("Success: %d, Errors: %d, Total: %d", successCount, errorCount, tc.connections)
			t.Logf("Success Rate: %.2f%%", float64(successCount)/float64(tc.connections)*100)

			if tc.expectError {
				assert.Greater(t, errorCount, 0, "Expected some errors for high connection count")
			} else {
				// Allow some errors due to system limits, but most should succeed
				successRate := float64(successCount) / float64(tc.connections)
				assert.Greater(t, successRate, 0.8, "Success rate should be > 80%")
			}

			// Check actual connections created
			impl := cmTest.(*packetConnManagerImpl)
			impl.connLock.RLock()
			actualConnections := len(impl.localConnections)
			impl.connLock.RUnlock()

			t.Logf("Actual connections created: %d", actualConnections)
		})
	}
}

// Benchmark tests for performance measurement

// BenchmarkDispatchNewConnection benchmarks creating new connections
func BenchmarkDispatchNewConnection(b *testing.B) {
	server, err := newMockServer()
	require.NoError(b, err)
	defer server.close()

	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		packet := &v1.Packet{
			ConnId:        int64(i + 1),
			Code:          v1.ControlCode_DATA,
			Data:          []byte("benchmark data"),
			TargetAddress: server.addr,
		}
		err := cm.Dispatch(packet)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDispatchExistingConnection benchmarks using existing connections
func BenchmarkDispatchExistingConnection(b *testing.B) {
	server, err := newMockServer()
	require.NoError(b, err)
	defer server.close()

	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	// Establish connection first
	packet := &v1.Packet{
		ConnId:        1,
		Code:          v1.ControlCode_DATA,
		Data:          []byte("initial"),
		TargetAddress: server.addr,
	}
	err = cm.Dispatch(packet)
	require.NoError(b, err)

	time.Sleep(50 * time.Millisecond) // Wait for connection

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		packet := &v1.Packet{
			ConnId:        1,
			Code:          v1.ControlCode_DATA,
			Data:          []byte("benchmark data"),
			TargetAddress: server.addr, // Should be ignored for existing connection
		}
		err := cm.Dispatch(packet)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestConcurrentDispatchPerformance tests concurrent dispatching performance
// This test creates many goroutines and should be run separately from unit tests
func TestConcurrentDispatchPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Start mock server
	server, err := newMockServer()
	require.NoError(t, err)
	defer server.close()

	ctx := context.Background()
	cm := newPacketConnectionManager(ctx)
	defer cm.Close()

	// Test with higher concurrency than the regular unit test
	const numGoroutines = 100
	const packetsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	start := time.Now()

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
	duration := time.Since(start)

	// Wait for all connections to be established
	time.Sleep(500 * time.Millisecond)

	// Verify all connections were created
	impl := cm.(*packetConnManagerImpl)
	impl.connLock.RLock()
	expectedConnections := numGoroutines * packetsPerGoroutine
	actualConnections := len(impl.localConnections)
	impl.connLock.RUnlock()

	t.Logf("Performance test completed:")
	t.Logf("- Goroutines: %d", numGoroutines)
	t.Logf("- Packets per goroutine: %d", packetsPerGoroutine)
	t.Logf("- Total packets: %d", expectedConnections)
	t.Logf("- Duration: %v", duration)
	t.Logf("- Packets per second: %.2f", float64(expectedConnections)/duration.Seconds())
	t.Logf("- Connections created: %d", actualConnections)

	assert.Equal(t, expectedConnections, actualConnections, "All connections should be created")
}
