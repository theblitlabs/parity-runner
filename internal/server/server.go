package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/core/config"
)

// Server handles HTTP requests for the runner API
type Server struct {
	httpServer  *http.Server
	mux         *http.ServeMux
	cfg         *config.Config
	controllers []Controller
}

// Controller defines an interface for server route controllers
type Controller interface {
	RegisterRoutes(mux *http.ServeMux)
}

// NewServer creates a new HTTP server instance
func NewServer(cfg *config.Config) *Server {
	mux := http.NewServeMux()

	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)

	return &Server{
		mux: mux,
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		cfg: cfg,
	}
}

// RegisterController adds a controller to the server
func (s *Server) RegisterController(c Controller) {
	s.controllers = append(s.controllers, c)
}

// Start initializes routes and starts the HTTP server
func (s *Server) Start() error {
	log := gologger.WithComponent("server")

	// Register all controllers
	for _, controller := range s.controllers {
		controller.RegisterRoutes(s.mux)
	}

	// Set up health check route
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Start the server
	serverAddr := s.httpServer.Addr
	log.Info().Str("addr", serverAddr).Msg("Starting HTTP server")

	return s.httpServer.ListenAndServe()
}

// Stop gracefully shuts down the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	log := gologger.WithComponent("server")
	log.Info().Msg("Shutting down HTTP server...")

	return s.httpServer.Shutdown(ctx)
}

// ServeMux returns the underlying HTTP mux for testing
func (s *Server) ServeMux() *http.ServeMux {
	return s.mux
}
