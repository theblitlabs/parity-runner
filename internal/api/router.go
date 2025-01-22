package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/virajbhartiya/parity-protocol/internal/api/handlers"
	"github.com/virajbhartiya/parity-protocol/internal/api/middleware"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Add origin check
		return true
	},
}

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

	// Create a separate router for WebSocket without middleware
	wsRouter := mux.NewRouter()
	wsRouter.HandleFunc(endpoint+"/runners/ws", r.handleWebSocket(taskHandler))

	// Combine the routers
	r.Router = r.Router.PathPrefix("/").Subrouter()
	r.Router.NotFoundHandler = wsRouter

	// Setup middleware for regular HTTP routes
	r.setup()
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
func (r *Router) registerRoutes(taskHandler *handlers.TaskHandler) {
	api := r.PathPrefix(r.endpoint).Subrouter()
	tasks := api.PathPrefix("/tasks").Subrouter()
	runners := api.PathPrefix("/runners").Subrouter()

	// Task routes (for task creators)
	tasks.HandleFunc("", taskHandler.CreateTask).Methods("POST")
	tasks.HandleFunc("", taskHandler.ListTasks).Methods("GET")
	tasks.HandleFunc("/{id}", taskHandler.GetTask).Methods("GET")
	tasks.HandleFunc("/{id}/assign", taskHandler.AssignTask).Methods("POST")
	tasks.HandleFunc("/{id}/reward", taskHandler.GetTaskReward).Methods("GET")
	tasks.HandleFunc("/{id}/result", taskHandler.GetTaskResult).Methods("GET")

	// Runner routes (for task executors)
	runners.HandleFunc("/tasks/available", taskHandler.ListAvailableTasks).Methods("GET")
	runners.HandleFunc("/tasks/{id}/start", taskHandler.StartTask).Methods("POST")
	runners.HandleFunc("/tasks/{id}/complete", taskHandler.CompleteTask).Methods("POST")
	runners.HandleFunc("/tasks/{id}/result", taskHandler.SaveTaskResult).Methods("POST")
}

func (r *Router) handleWebSocket(taskHandler *handlers.TaskHandler) http.HandlerFunc {
	log := logger.Get()
	log.Info().Msg("WebSocket handler registered")
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error(err, "WebSocket upgrade failed")
			return
		}
		defer conn.Close()

		taskHandler.HandleWebSocket(conn)
	}
}

// AddMiddleware adds a new middleware to the router
func (r *Router) AddMiddleware(middleware mux.MiddlewareFunc) {
	r.Use(middleware)
}
