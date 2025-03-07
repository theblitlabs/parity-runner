package api

import (
	"github.com/gorilla/mux"
	"github.com/theblitlabs/parity-protocol/internal/api/handlers"
	"github.com/theblitlabs/parity-protocol/internal/api/middleware"
)

// Router wraps mux.Router to add more functionality
type Router struct {
	*mux.Router
	middleware []mux.MiddlewareFunc
	endpoint   string
}

// NewRouter creates and configures a new router with all dependencies
func NewRouter(
	taskHandler *handlers.TaskHandler,
	endpoint string,
) *Router {
	r := &Router{
		Router:     mux.NewRouter(),
		middleware: []mux.MiddlewareFunc{middleware.Logging},
		endpoint:   endpoint,
	}

	// Setup middleware for regular HTTP routes
	r.setup()

	// Create API subrouter with middleware
	apiRouter := r.Router.PathPrefix("/").Subrouter()
	for _, m := range r.middleware {
		apiRouter.Use(m)
	}

	// Register HTTP routes on the API subrouter
	r.registerRoutes(apiRouter, taskHandler)

	return r
}

// setup configures the base router with middleware and common settings
func (r *Router) setup() {
	// Apply global middleware
	for _, m := range r.middleware {
		r.Use(m)
	}
}

// registerRoutes registers all application routes
func (r *Router) registerRoutes(router *mux.Router, taskHandler *handlers.TaskHandler) {
	api := router.PathPrefix(r.endpoint).Subrouter()
	tasks := api.PathPrefix("/tasks").Subrouter()
	runners := api.PathPrefix("/runners").Subrouter()

	// Task routes (for task creators)
	tasks.HandleFunc("", taskHandler.CreateTask).Methods("POST")
	tasks.HandleFunc("", taskHandler.ListTasks).Methods("GET")
	tasks.HandleFunc("/{id}", taskHandler.GetTask).Methods("GET")
	tasks.HandleFunc("/{id}/assign", taskHandler.AssignTask).Methods("POST")
	tasks.HandleFunc("/{id}/reward", taskHandler.GetTaskReward).Methods("GET")
	tasks.HandleFunc("/{id}/result", taskHandler.GetTaskResult).Methods("GET")
	tasks.HandleFunc("/{id}/status", taskHandler.UpdateTaskStatus).Methods("PUT")

	// Runner routes (for task executors)
	runners.HandleFunc("/tasks/available", taskHandler.ListAvailableTasks).Methods("GET")
	runners.HandleFunc("/tasks/{id}/start", taskHandler.StartTask).Methods("POST")
	runners.HandleFunc("/tasks/{id}/complete", taskHandler.CompleteTask).Methods("POST")
	runners.HandleFunc("/tasks/{id}/result", taskHandler.SaveTaskResult).Methods("POST")

	// Webhook routes
	runners.HandleFunc("/webhooks", taskHandler.RegisterWebhook).Methods("POST")
	runners.HandleFunc("/webhooks/{id}", taskHandler.UnregisterWebhook).Methods("DELETE")
}

// AddMiddleware adds a new middleware to the router
func (r *Router) AddMiddleware(middleware mux.MiddlewareFunc) {
	r.Use(middleware)
}
