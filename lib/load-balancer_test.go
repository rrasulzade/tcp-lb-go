package lib

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackendConnectionCountUpdates(t *testing.T) {
	require := require.New(t)

	backend := &Backend{Address: "127.0.0.1:5001"}

	backend.incrementConnections()
	require.Equal(int64(1), backend.ConnectionCount(), "Expected connection count to be 1")

	backend.decrementConnections()
	require.Equal(int64(0), backend.ConnectionCount(), "Expected connection count to be 0")
}

func TestBackendConcurrentConnectionCountUpdates(t *testing.T) {
	require := require.New(t)

	backend := &Backend{Address: "127.0.0.1:5001"}

	var wg sync.WaitGroup
	numRoutines := 100
	updatesPerRoutine := 10

	// Concurrently update the connection count
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < updatesPerRoutine; j++ {
				backend.incrementConnections()
				backend.decrementConnections()
			}
		}()
	}
	// Wait for all goroutines to complete
	wg.Wait()
	require.Equal(int64(0), backend.ConnectionCount(), "Expected connection count to be 0")
}

func TestLoadBalancer(t *testing.T) {
	require := require.New(t)

	defaultCapacity := uint64(5)
	defaulRefillRate := uint64(1)
	backend1 := Backend{Address: "127.0.0.1:5001"}
	backend2 := Backend{Address: "127.0.0.1:5002"}

	t.Run("Initialization", func(t *testing.T) {
		lb := NewLoadBalancer(defaultCapacity, defaulRefillRate)
		require.Equal(0, len(lb.backends), "Expected 0 backends")
	})

	t.Run("Add backend to LoadBalancer", func(t *testing.T) {
		lb := NewLoadBalancer(defaultCapacity, defaulRefillRate)

		lb.AddBackend(&backend1)
		require.Equal(1, len(lb.backends), "Expected 1 backend")
	})

	t.Run("Retrieve backend from LoadBalancer", func(t *testing.T) {
		lb := NewLoadBalancer(defaultCapacity, defaulRefillRate)

		lb.AddBackend(&backend1)
		lb.AddBackend(&backend2)

		allowedBackends := map[string]struct{}{
			backend1.Address: {},
			backend2.Address: {},
		}
		b, err := lb.GetBackend(allowedBackends)
		require.NoError(err)
		require.Equal(backend1.Address, b.Address, "Expected backend1")

		b.incrementConnections()

		b, err = lb.GetBackend(allowedBackends)
		require.NoError(err)
		require.Equal(backend2.Address, b.Address, "Expected backend2")

		b.incrementConnections()
		b.incrementConnections()

		b, err = lb.GetBackend(allowedBackends)
		require.NoError(err)
		require.Equal(backend1.Address, b.Address, "Expected backend1")
	})

	t.Run("No registered backends", func(t *testing.T) {
		lb := NewLoadBalancer(defaultCapacity, defaulRefillRate)

		allowedBackends := map[string]struct{}{
			backend1.Address: {},
		}
		_, err := lb.GetBackend(allowedBackends)
		require.ErrorIs(ErrNoRegisteredBackends, err, "Expected ErrNoRegisteredBackends error")
	})

	t.Run("No available backends", func(t *testing.T) {
		lb := NewLoadBalancer(defaultCapacity, defaulRefillRate)

		lb.AddBackend(&backend1)

		allowedBackends := map[string]struct{}{
			backend2.Address: {},
		}
		_, err := lb.GetBackend(allowedBackends)
		require.ErrorIs(ErrNoAvailableBackend, err, "Expected ErrNoAvailableBackend")
	})

	t.Run("Concurrent AddBackend", func(t *testing.T) {
		lb := NewLoadBalancer(defaultCapacity, defaulRefillRate)

		var wg sync.WaitGroup
		numRoutines := 100

		// Concurrently add backends
		for i := 0; i < numRoutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				backend := &Backend{Address: fmt.Sprintf("127.0.0.1:500%d", id)}
				require.NotPanics(func() {
					lb.AddBackend(backend)
				}, "Panic occurred during concurrent access.")
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()

		require.Equal(numRoutines, len(lb.backends), fmt.Sprintf("Expected %d backends", numRoutines))
	})

	t.Run("Concurrent GetBackend", func(t *testing.T) {
		lb := NewLoadBalancer(defaultCapacity, defaulRefillRate)
		numBackends := 10
		for i := 0; i < numBackends; i++ {
			backend := &Backend{Address: fmt.Sprintf("127.0.0.1:500%d", i)}
			lb.AddBackend(backend)
		}

		allowedBackends := map[string]struct{}{
			"127.0.0.1:5001": {},
			"127.0.0.1:5002": {},
			"127.0.0.1:5003": {},
		}

		var wg sync.WaitGroup
		numRoutines := 100

		// Concurrently get backends
		for i := 0; i < numRoutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				require.NotPanics(func() {
					b, err := lb.GetBackend(allowedBackends)
					require.NoError(err)
					b.incrementConnections()
				}, "Panic occurred during concurrent access.")
			}()
		}

		// Wait for all goroutines to complete
		wg.Wait()
	})
}

// Mock connection for testing
type mockConn struct {
	readBuffer  *bytes.Buffer
	writeBuffer *bytes.Buffer
}

func (mc *mockConn) Read(b []byte) (n int, err error)   { return mc.readBuffer.Read(b) }
func (mc *mockConn) Write(b []byte) (n int, err error)  { return mc.writeBuffer.Write(b) }
func (mc *mockConn) Close() error                       { return nil }
func (mc *mockConn) LocalAddr() net.Addr                { return nil }
func (mc *mockConn) RemoteAddr() net.Addr               { return nil }
func (mc *mockConn) SetDeadline(t time.Time) error      { return nil }
func (mc *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (mc *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// Mock dialer for testing
type mockDialer struct{}

func (d *mockDialer) Dial(network, address string) (net.Conn, error) {
	return &mockConn{
		readBuffer:  bytes.NewBuffer([]byte("mock data")),
		writeBuffer: new(bytes.Buffer),
	}, nil
}

func TestRouteConnection(t *testing.T) {
	require := require.New(t)

	defaultCapacity := uint64(5)
	defaulRefillRate := uint64(5)

	lb := NewLoadBalancer(defaultCapacity, defaulRefillRate)
	lb.dialer = &mockDialer{}

	backend := &Backend{Address: "127.0.0.1:5010"}
	lb.AddBackend(backend)

	allowedBackends := map[string]struct{}{
		backend.Address: {},
	}

	// Mock client connection
	clientMockConn := &mockConn{
		readBuffer:  bytes.NewBuffer([]byte("client data")),
		writeBuffer: new(bytes.Buffer),
	}

	err := lb.RouteConnection("client1", clientMockConn, allowedBackends)
	require.NoError(err)
	require.Equal(int64(0), backend.ConnectionCount(), "Expected connection count to be 0")

	// Test rate limiting by exceeding the allowed rate
	for i := 0; i < 10; i++ {
		err = lb.RouteConnection("client1", clientMockConn, allowedBackends)
	}
	require.ErrorIs(ErrRateLimitReached, err, "Expected rate limit error")
}
