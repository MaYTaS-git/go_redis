package server

import (
	"crypto/tls"
	"fmt"
	"net"
)

// ListenTLS initializes a TLS network listener using the provided cert and key files.
func ListenTLS(addr string, certFile string, keyFile string) (net.Listener, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificate key pair: %w", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	listener, err := tls.Listen("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on TLS addr %s: %w", addr, err)
	}

	return listener, nil
}
