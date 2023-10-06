package server

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rrasulzade/tcp-lb-go/lib"
)

// ServerConfig encapsulates the configuration parameters required
// to initialize and run the server.
type ServerConfig struct {
	// Address is an address on which the server listens.
	Address string

	// LoadBalancer is LoadBalancer instance to distribute incoming connections.
	LoadBalancer *lib.LoadBalancer

	// TLSConfig represents TLS configurations.
	TLSConfig *tls.Config

	// AllowedClients is a map of clients that are allowed to connect.
	AllowedClients map[string]bool

	// ClientBackendACL defines the access control list for clients and backends.
	ClientBackendACL map[string]map[string]struct{}
}

// Server represents the main structure for the load balancer server.
type Server struct {
	// mu ensures concurrent access.
	mu sync.RWMutex

	// config is configuration object that holds all the server settings.
	config *ServerConfig

	// listener accepts incoming connections.
	listener net.Listener

	// shutdown is an atomic boolean to signal server shutdown.
	shutdown atomic.Bool

	// done is a WaitGroup to wait for goroutines to finish.
	wg sync.WaitGroup

	// connection is a channel to handle incoming connections.
	connection chan net.Conn
}

// NewServer creates a new Server instance.
func NewServer(config *ServerConfig) (*Server, error) {
	// Check if the provided address is valid
	if config.Address == "" {
		return nil, errors.New("provided address is blank")
	}

	// Check if the provided load balancer instance is valid
	if config.LoadBalancer == nil {
		return nil, errors.New("load balancer instance is required")
	}

	// Check if the provided TLS configuration is valid
	if config.TLSConfig == nil {
		return nil, errors.New("TLS configuration is required")
	}
	if len(config.AllowedClients) == 0 {
		return nil, errors.New("allowed clients list configuration is required")
	}
	if len(config.ClientBackendACL) == 0 {
		return nil, errors.New("access control list configuration is required")
	}

	return &Server{
		config:     config,
		connection: make(chan net.Conn),
	}, nil
}

// acceptConnections listens and accepts incoming requests.
// TODO: add custom logger that supports log levels for debugging
func (s *Server) acceptConnections() {
	defer s.wg.Done()

	log.Printf("Server is listening on %s\n", s.config.Address)

	// TODO: add retryLimit and retryDelay settings to the config structure
	retryLimit := 5
	retryDelay := time.Second

	retryCount := 0
	for !s.shutdown.Load() {
		conn, err := s.listener.Accept()
		if err != nil {
			if retryCount < retryLimit {
				retryCount++
				log.Printf("Error accepting connection: %v\n", err)
				time.Sleep(retryDelay)
				continue
			}
			// TODO: replace with a proper notification or monitoring mechanism to notify maintainers
			log.Fatalf("Exiting due to repeated errors: %v", err)
		}
		// reset retry counter
		retryCount = 0
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			err := s.handleConnection(conn)
			if err != nil {
				log.Printf("Error handling connection from %s: %v", conn.RemoteAddr(), err)
			}
		}()
	}
}

// handleConnection handles incoming connections individually
// by forwarding them to the selected backend server.
// TODO: add custom logger that supports log levels for debugging
func (s *Server) handleConnection(clientConn net.Conn) error {
	defer clientConn.Close()

	// Authenticate client connection using TLS
	clientCert, err := AuthenticateClient(clientConn, s.config.AllowedClients)
	if err != nil {
		return fmt.Errorf("TLS authentication failed for incoming connection: %w", err)
	}

	// Generate client ID based on the client's certificate details
	clientID := GenerateClientID(clientCert.Subject.CommonName, clientCert.SerialNumber.String())

	// Authorize the client to grant access
	allowedBackends, err := AuthorizeClient(clientID, s.config.ClientBackendACL)
	if err != nil {
		return fmt.Errorf("authorization denied for client with CN=%s err: %w", clientCert.Subject.CommonName, err)
	}

	// Forward the connection to the appropriate backend server
	err = s.config.LoadBalancer.RouteConnection(clientID, clientConn, allowedBackends)
	if err != nil {
		return fmt.Errorf("unable to forward connection to backend server: %w", err)
	}

	return nil
}

// Start initializes the server listener and starts the main server.
func (s *Server) Start() error {
	var err error

	s.listener, err = tls.Listen("tcp", s.config.Address, s.config.TLSConfig)
	if err != nil {
		return fmt.Errorf("unable to initialize server TLS listener: %w", err)
	}

	s.wg.Add(1)
	go s.acceptConnections()

	return nil
}

// Stop shuts down the load balancer server gracefully.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.shutdown.Store(true)
	s.listener.Close()

	done := make(chan struct{})
	// Start a goroutine to wait for all active connections to finish
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(time.Second):
		return errors.New("server shutdown timed out waiting for connections to close")
	}

}

// GenerateClientID creates a clientID by hashing the provided
// CommonName and SerialNumber using SHA-256 alg
func GenerateClientID(cn string, serialNumber string) string {
	// Concatenate CN and serial number with a separator ':' in between
	combined := fmt.Sprintf("%s:%s", cn, serialNumber)

	// Generate a SHA-256 hash of the combined string
	hash := sha256.Sum256([]byte(combined))

	// Convert the hash to a string
	clientID := hex.EncodeToString(hash[:])

	return clientID
}

// AuthorizeClient checks if the provided client is authorized to access backends.
// Returns the list of allowed backends for the client.
func AuthorizeClient(
	clientID string,
	clientBackendACL map[string]map[string]struct{},
) (map[string]struct{}, error) {
	allowedBackends, ok := clientBackendACL[clientID]
	if !ok {
		return nil, fmt.Errorf("client %s is not listed in the provided access control list", clientID)
	}
	return allowedBackends, nil
}

// GetTLSConnection ensures the connection is a TLS connection.
func GetTLSConnection(clientConn net.Conn) (*tls.Conn, error) {
	tlsConn, ok := clientConn.(*tls.Conn)
	if !ok {
		return nil, fmt.Errorf("expected a TLS connection")
	}
	return tlsConn, nil
}

// GetClientCertificate retrieves the client's certificate from the connection.
func GetClientCertificate(tlsConn *tls.Conn) (*x509.Certificate, error) {
	clientCerts := tlsConn.ConnectionState().PeerCertificates
	if len(clientCerts) == 0 {
		return nil, fmt.Errorf("client did not provide a TLS certificate")
	}
	return clientCerts[0], nil
}

// ValidateCommonName checks if the CommonName (CN) from
// the client's certificate is in the allowed list.
func ValidateCommonName(clientCert *x509.Certificate, allowedClients map[string]bool) error {
	clientCertCN := clientCert.Subject.CommonName
	if clientCertCN == "" {
		return fmt.Errorf("client's TLS certificate lacks a CommonName")
	}
	_, isAllowed := allowedClients[clientCertCN]
	if !isAllowed {
		return fmt.Errorf("client with CommonName %s is not allowed", clientCertCN)
	}
	return nil
}

// AuthenticateClient verifies the client's certificate CN.
// Returns client's verified certificate
func AuthenticateClient(clientConn net.Conn, allowedClients map[string]bool) (*x509.Certificate, error) {
	tlsConn, err := GetTLSConnection(clientConn)
	if err != nil {
		return nil, err
	}

	err = tlsConn.Handshake()
	if err != nil {
		return nil, err
	}

	clientCert, err := GetClientCertificate(tlsConn)
	if err != nil {
		return nil, err
	}

	err = ValidateCommonName(clientCert, allowedClients)
	if err != nil {
		return nil, err
	}

	return clientCert, nil
}
