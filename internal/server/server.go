package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/parity-runner/internal/core/config"
)

type Server struct {
	httpServer  *http.Server
	mux         *http.ServeMux
	cfg         *config.Config
	controllers []Controller
}

type Controller interface {
	RegisterRoutes(mux *http.ServeMux)
}

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

func (s *Server) RegisterController(c Controller) {
	s.controllers = append(s.controllers, c)
}

func (s *Server) Start() error {
	log := gologger.WithComponent("server")

	for _, controller := range s.controllers {
		controller.RegisterRoutes(s.mux)
	}

	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	serverAddr := s.httpServer.Addr
	log.Info().Str("addr", serverAddr).Msg("Starting HTTP server")

	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	log := gologger.WithComponent("server")
	log.Info().Msg("Shutting down HTTP server...")

	return s.httpServer.Shutdown(ctx)
}

func (s *Server) ServeMux() *http.ServeMux {
	return s.mux
}
