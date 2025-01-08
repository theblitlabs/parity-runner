package api

import (
	"github.com/gorilla/mux"
	"github.com/virajbhartiya/parity-protocol/internal/api/handlers"
	"github.com/virajbhartiya/parity-protocol/internal/api/middleware"
)

func NewRouter(taskHandler *handlers.TaskHandler) *mux.Router {
	r := mux.NewRouter()

	// Apply middlewares
	r.Use(middleware.Logging) // Add logging middleware
	r.Use(middleware.Auth)

	// Task routes
	r.HandleFunc("/api/tasks", taskHandler.CreateTask).Methods("POST")
	r.HandleFunc("/api/tasks/{id}", taskHandler.GetTask).Methods("GET")
	r.HandleFunc("/api/tasks", taskHandler.ListTasks).Methods("GET")

	return r
}
