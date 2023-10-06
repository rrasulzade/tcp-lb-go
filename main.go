package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rrasulzade/tcp-lb-go/config"
	"github.com/rrasulzade/tcp-lb-go/lib"
	"github.com/rrasulzade/tcp-lb-go/server"
)

// TODO: add custom logger that supports log levels for debugging
func main() {
	// Define a custom flag usage function
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		flag.PrintDefaults()
	}
	// Read config file flag
	var configFileFlag string
	flag.StringVar(&configFileFlag, "config", "", "Path to a configuration file")
	flag.Parse()

	// Check if the config flag was provided
	if configFileFlag == "" {
		fmt.Println("Error: Configuration file not provided")
		flag.Usage()
		return
	}

	// Load gloabal AppConfig settings
	appConfig, err := config.LoadAppConfig(configFileFlag)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the load balancer
	lb := lib.NewLoadBalancer(
		appConfig.RateLimiter.Capacity,
		appConfig.RateLimiter.RefillRate)

	// Add backend servers to the load balancer
	log.Println("Backend Servers:")
	for i, address := range appConfig.Backends {
		server := &lib.Backend{
			Address: address,
		}
		lb.AddBackend(server)
		// Print the backend server addr
		log.Printf("%d: %s\n", i+1, address)
	}

	// Configure TLS options
	tlsConfig, err := config.MakeServerTLSConfig(
		appConfig.TLS.CertFile,
		appConfig.TLS.KeyFile,
		appConfig.TLS.CAFile)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize the server
	listenAddr := fmt.Sprintf(":%d", appConfig.Port)
	serverConfig := &server.ServerConfig{
		Address:          listenAddr,
		LoadBalancer:     lb,
		TLSConfig:        tlsConfig,
		AllowedClients:   appConfig.AllowedClients,
		ClientBackendACL: mapSliceToMapSet(appConfig.ClientBackendACL),
	}
	lbServer, err := server.NewServer(serverConfig)
	if err != nil {
		log.Fatal(err)
	}

	// Start the server
	err = lbServer.Start()
	if err != nil {
		log.Fatal(err)
	}

	// Wait for a SIGINT or SIGTERM signal to gracefully shut down the server
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down the server...")

	// Stop the server
	err = lbServer.Stop()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Server stopped.")
}

// mapSliceToMapSet converts a map of slices to a map of sets.
func mapSliceToMapSet(mapSlice map[string][]string) map[string]map[string]struct{} {
	mapSet := make(map[string]map[string]struct{}, len(mapSlice))
	for key, slice := range mapSlice {
		set := make(map[string]struct{}, len(slice))
		for _, item := range slice {
			set[item] = struct{}{}
		}
		mapSet[key] = set
	}
	return mapSet
}
