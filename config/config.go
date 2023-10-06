package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// RateLimiterConfig defines the rate limiting settings.
type RateLimiterConfig struct {
	// Capacity is the maximum number of tokens in the bucket.
	Capacity uint64 `json:"capacity"`

	// RefillRate is the number of tokens added to the bucket every second.
	RefillRate uint64 `json:"refill_rate"`
}

// TLSConfig defines the TLS settings.
type TLSConfig struct {
	// CertFile is a path to a server certificate file.
	CertFile string `json:"cert_file"`

	// KeyFile is a path to a server private key file.
	KeyFile string `json:"key_file"`

	// CAFile is a path to a root CA file.
	CAFile string `json:"ca_file"`
}

// ApplicationConfig holds all the configuration settings.
type ApplicationConfig struct {
	// Port is a port number on which the server runs.
	Port int `json:"port"`

	// Backends is a list of backends to add to the load balancer.
	Backends []string `json:"backends"`

	// TLS is TLS configuration settings.
	TLS *TLSConfig `json:"tls"`

	// RateLimiter is the rate limiting settings.
	RateLimiter RateLimiterConfig `json:"rate_limiter"`

	// AllowedClients is a map of clients that are allowed to connect.
	AllowedClients map[string]bool `json:"allowed_clients"`

	// ClientBackendACL defines the access control list for clients and backends.
	ClientBackendACL map[string][]string `json:"client_backend_acl"`
}

// LoadAppConfig reads the configuration from a JSON file and
// unmarshals it into the global AppConfig variable.
func LoadAppConfig(configFile string) (*ApplicationConfig, error) {
	// Initialize default settings
	appConfig := &ApplicationConfig{
		Port: 3003,
		RateLimiter: RateLimiterConfig{
			Capacity:   10,
			RefillRate: 2,
		},
		AllowedClients:   make(map[string]bool),
		ClientBackendACL: make(map[string][]string),
	}

	// Open configurations JSON file
	f, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("unable to open configurations file '%s': %w", configFile, err)
	}

	if err := json.NewDecoder(f).Decode(appConfig); err != nil {
		return nil, fmt.Errorf("configuration parsing error for file '%s': %w", configFile, err)
	}

	// Verify if required values are provided
	if appConfig.TLS == nil {
		return nil, errors.New("TLS configuration is required")
	}
	if len(appConfig.Backends) == 0 {
		return nil, errors.New("backend service configuration is required")
	}
	if len(appConfig.AllowedClients) == 0 {
		return nil, errors.New("allowed clients list configuration is required")
	}
	if len(appConfig.ClientBackendACL) == 0 {
		return nil, errors.New("access control list configuration is required")
	}
	return appConfig, nil
}

// MakeServerTLSConfig creates a TLS configuration using the provided certificate,
// key, and CA files and ensures that only TLS 1.3 is used,
// requires and verifies client certificates for mutual TLS authentication.
// It returns a configured tls.Config object.
func MakeServerTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	// Load the certificate and private key
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("unable to load server certificate and key: %w", err)
	}

	// Read the CA certificate file
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read CA certificate: %w", err)
	}

	// Create a new certificate pool and add the CA certificate to it
	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caCert)
	if !ok {
		return nil, errors.New("unable to parse CA certificate PEM")
	}

	// Construct the TLS configuration
	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
	}
	return tlsConfig, nil
}
