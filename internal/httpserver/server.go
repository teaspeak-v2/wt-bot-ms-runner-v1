package httpserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Server wraps an HTTP server.
type Server struct {
	server          *http.Server
	ln              net.Listener
	shutdownTimeout time.Duration
}

// New creates a new HTTP server.
func New(addr string, handler http.Handler, readTimeout, writeTimeout, idleTimeout, shutdownTimeout time.Duration) *Server {
	return &Server{
		server: &http.Server{
			Addr:        addr,
			Handler:     handler,
			ReadTimeout: readTimeout,
			WriteTimeout: writeTimeout,
			IdleTimeout: idleTimeout,
		},
		shutdownTimeout: shutdownTimeout,
	}
}

// Run starts the server and blocks until the context is cancelled.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.ln = ln

	errCh := make(chan error, 1)
	go func() { errCh <- s.server.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
