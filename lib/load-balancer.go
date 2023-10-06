package lib

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"
)

// define custom errors.
var (
	ErrNoRegisteredBackends = errors.New("no registered backends")
	ErrNoAvailableBackend   = errors.New("no available backend")
	ErrRateLimitReached     = errors.New("connection rejected due to rate limiting")
)

// dialer is an interface that abstracts the Dial method.
type dialer interface {
	Dial(network, address string) (net.Conn, error)
}

// lbDialer is the default implementation of the dialer interface.
type lbDialer struct{}

func (d *lbDialer) Dial(network, address string) (net.Conn, error) {
	return net.Dial(network, address)
}

// Backend represents a backend server that
// the load balancer can forward requests to.
type Backend struct {
	// Address is a hostname or IP address of the backend server.
	Address string

	// connections is the current number of active connections.
	connections atomic.Int64
}

// incrementConnections increments the active connection count by one.
func (b *Backend) incrementConnections() {
	b.connections.Add(1)
}

// decrementConnections decrements the active connection count by one.
func (b *Backend) decrementConnections() {
	b.connections.Add(-1)
}

// ConnectionCount returns the active connection count.
func (b *Backend) ConnectionCount() int64 {
	return b.connections.Load()
}

// LoadBalancer is responsible for managing a list of
// backend servers and forwarding incoming requests
// to them by leveraging least connections algorithm.
type LoadBalancer struct {
	// mu ensures concurrent access to the backends list.
	mu sync.RWMutex

	// backends is a list of registered backends ready to accept requests.
	backends []*Backend

	// rateLimiter controls the rate of incoming connections.
	rateLimiter *rateLimiter

	// dialer is a dialer interface to establish backend connections.
	dialer dialer
}

// NewLoadBalancer initializes and returns a new LoadBalancer.
func NewLoadBalancer(bucketCapacity, bucketRefillRate uint64) *LoadBalancer {
	// Initialize the rate limiter
	rl := newRateLimiter(bucketCapacity, bucketRefillRate)

	return &LoadBalancer{
		rateLimiter: rl,
		dialer:      &lbDialer{},
	}
}

// AddBackend adds a backend server to the load balancer.
func (lb *LoadBalancer) AddBackend(backend *Backend) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.backends = append(lb.backends, backend)
}

// GetBackend returns a backend server with the least connections by
// iterating through the provided available backend servers pool and
// matching with the provided list of allowed backends for the client.
// It increments the connection count for the chosen backend before returning it.
func (lb *LoadBalancer) GetBackend(allowedBackends map[string]struct{}) (*Backend, error) {
	// Acquire the lock
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// No registered backend servers
	if len(lb.backends) == 0 {
		return nil, ErrNoRegisteredBackends
	}

	var selectedBackend *Backend
	var leastConnectionCount int64
	for _, backend := range lb.backends {
		// Check if the backend is allowed for the client
		if _, exists := allowedBackends[backend.Address]; !exists {
			continue
		}

		// Find the backend server with the least connections
		if selectedBackend == nil ||
			backend.ConnectionCount() < leastConnectionCount {
			selectedBackend = backend
			leastConnectionCount = backend.ConnectionCount()
		}
	}

	// No available backend
	if selectedBackend == nil {
		return nil, ErrNoAvailableBackend
	}

	// Increment the connection count for the selected backend server
	selectedBackend.incrementConnections()

	return selectedBackend, nil
}

// RouteConnection handles the routing of a client connection
// to an appropriate backend server.
func (lb *LoadBalancer) RouteConnection(
	clientID string,
	clientConn net.Conn,
	allowedBackends map[string]struct{}) error {
	// Check for rate limiting whether the client has sufficient tokens
	if !lb.rateLimiter.allowConnection(clientID) {
		return ErrRateLimitReached
	}

	// Select a backend server with the least connections
	selectedBackend, err := lb.GetBackend(allowedBackends)
	if err != nil {
		return err
	}

	// To accurately select a backend with the least connections,
	// the connection count for each backend server has to be up-to-date
	// and accurately reflect the current number of active connections.
	// While both, incrementConnections() and decrementConnections() are
	// atomic operations for each backend server, the mutex is used to
	// prevent race conditions arising within the load balancer.
	defer func() {
		lb.mu.Lock()
		// Decrement the connection count for the selected backend server
		selectedBackend.decrementConnections()
		lb.mu.Unlock()
	}()

	// Establish a connection to the selected backend server
	backendConn, err := lb.dialer.Dial("tcp", selectedBackend.Address)
	if err != nil {
		return err
	}
	defer backendConn.Close()

	// Bidirectional data transfer between the client and backend server.
	// Waits till both sides complete copying data
	err = transferData(clientConn, backendConn)
	if err != nil {
		return err
	}

	return nil
}
