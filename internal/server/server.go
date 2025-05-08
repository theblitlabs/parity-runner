package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/core/config"
)

type Server struct {
	httpServer  *http.Server
	router      *gin.Engine
	cfg         *config.Config
	controllers []Controller
}

type Controller interface {
	RegisterRoutes(router *gin.Engine)
}

func NewServer(cfg *config.Config) *Server {
	// Set Gin mode based on configuration
	gin.SetMode(gin.ReleaseMode) // Set to gin.DebugMode for development

	router := gin.New()
	
	// Add middleware
	router.Use(gin.Recovery())
	
	// Add a custom logger middleware using your existing logger
	router.Use(func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		
		c.Next()
		
		end := time.Now()
		latency := end.Sub(start)
		
		log := gologger.WithComponent("gin")
		log.Info().
			Str("method", c.Request.Method).
			Str("path", path).
			Int("status", c.Writer.Status()).
			Dur("latency", latency).
			Str("ip", c.ClientIP()).
			Msg("Request")
	})

	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)

	return &Server{
		router: router,
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      router,
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
		controller.RegisterRoutes(s.router)
	}

	// Add health check endpoint
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
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

func (s *Server) Router() *gin.Engine {
	return s.router
}
