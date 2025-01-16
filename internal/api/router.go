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
	// Tasks
	r.registerTaskRoutes(taskHandler)

}

// registerTaskRoutes registers all task-related routes
func (r *Router) registerTaskRoutes(h *handlers.TaskHandler) {
	// Create API version subrouter
	api := r.PathPrefix(r.endpoint).Subrouter()

	// Create tasks subrouter under the API version
	tasks := api.PathPrefix("/tasks").Subrouter()

	// Task routes
	tasks.HandleFunc("", h.CreateTask).Methods("POST")
	tasks.HandleFunc("/{id}", h.GetTask).Methods("GET")
	tasks.HandleFunc("", h.ListTasks).Methods("GET")
	tasks.HandleFunc("/{id}/assign", h.AssignTask).Methods("POST")
	tasks.HandleFunc("/{id}/reward", h.GetTaskReward).Methods("GET")
}

// AddMiddleware adds a new middleware to the router
func (r *Router) AddMiddleware(middleware mux.MiddlewareFunc) {
	r.Use(middleware)
}
