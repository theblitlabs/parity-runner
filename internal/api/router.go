package api

import (
	"github.com/gorilla/mux"
	"github.com/virajbhartiya/parity-protocol/internal/api/handlers"
	"github.com/virajbhartiya/parity-protocol/internal/api/middleware"
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
		Router: mux.NewRouter(),
		middleware: []mux.MiddlewareFunc{
			middleware.Logging,
			// Temporarily comment out auth for testing
			// middleware.Auth,
		},
		endpoint: endpoint,
	}

	// Initialize the router
	r.setup()

	// Register all routes
	r.registerRoutes(taskHandler)

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
func (r *Router) registerRoutes(
	taskHandler *handlers.TaskHandler,
	// Add new handlers here as parameters when needed:
) {
	// Create API version subrouter
	api := r.PathPrefix(r.endpoint).Subrouter()

	// Create separate subrouters for tasks and runners
	tasks := api.PathPrefix("/tasks").Subrouter()
	runners := api.PathPrefix("/runners").Subrouter()

	// Task routes (for task creators)
	tasks.HandleFunc("", taskHandler.CreateTask).Methods("POST")
	tasks.HandleFunc("", taskHandler.ListTasks).Methods("GET")
	tasks.HandleFunc("/{id}", taskHandler.GetTask).Methods("GET")
	tasks.HandleFunc("/{id}/assign", taskHandler.AssignTask).Methods("POST")
	tasks.HandleFunc("/{id}/reward", taskHandler.GetTaskReward).Methods("GET")

	// Runner routes (for task executors)
	runners.HandleFunc("/tasks/available", taskHandler.ListAvailableTasks).Methods("GET")
	runners.HandleFunc("/tasks/{id}/start", taskHandler.StartTask).Methods("POST")
	runners.HandleFunc("/tasks/{id}/complete", taskHandler.CompleteTask).Methods("POST")
}

// AddMiddleware adds a new middleware to the router
func (r *Router) AddMiddleware(middleware mux.MiddlewareFunc) {
	r.Use(middleware)
}
