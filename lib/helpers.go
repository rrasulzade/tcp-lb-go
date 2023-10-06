package lib

import (
	"errors"
	"fmt"
	"io"
	"net"
)

// TransferData bidirectionally transfers data between a client and backend connections
func transferData(clientConn, backendConn net.Conn) error {
	errChan := make(chan error, 2)

	// Goroutine to handle data transfer from the backend to the client
	go func() {
		_, err := io.Copy(clientConn, backendConn)
		if err != nil {
			errChan <- fmt.Errorf("copying data from backend server: %w", err)
		} else {
			errChan <- nil
		}
	}()

	// Goroutine to handle data transfer from the client to the backend
	go func() {
		_, err := io.Copy(backendConn, clientConn)
		if err != nil {
			errChan <- fmt.Errorf("copying data to backend server: %w", err)
		} else {
			errChan <- nil
		}
	}()

	// Wait for both goroutines to complete
	err1 := <-errChan
	err2 := <-errChan

	return errors.Join(err1, err2)
}
