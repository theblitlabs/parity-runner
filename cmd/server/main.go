package main

import (
	"log"
	"net/http"

	"github.com/virajbhartiya/parity-protocol/internal/api"
	"github.com/virajbhartiya/parity-protocol/internal/api/handlers"
	"github.com/virajbhartiya/parity-protocol/internal/config"
	"github.com/virajbhartiya/parity-protocol/internal/database"
	"github.com/virajbhartiya/parity-protocol/internal/database/repositories"
	"github.com/virajbhartiya/parity-protocol/internal/services"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := database.NewDB(&cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize repositories
	taskRepo := repositories.NewTaskRepository(db)

	// Initialize services
	taskService := services.NewTaskService(taskRepo)

	// Initialize handlers
	taskHandler := handlers.NewTaskHandler(taskService)

	// Initialize router
	router := api.NewRouter(taskHandler)

	// Start server
	addr := cfg.Server.Host + ":" + cfg.Server.Port
	log.Printf("Server starting on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
