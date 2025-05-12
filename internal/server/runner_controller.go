package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/theblitlabs/gologger"

	"github.com/theblitlabs/parity-runner/internal/core/models"
	"github.com/theblitlabs/parity-runner/internal/core/services"
)

type RunnerController struct {
	runnerService services.RunnerService
}

func NewRunnerController(runnerService services.RunnerService) *RunnerController {
	return &RunnerController{
		runnerService: runnerService,
	}
}

func (c *RunnerController) RequireDeviceID(ctx *gin.Context) {
	log := gologger.WithComponent("runner_controller")

	deviceID := ctx.GetHeader("X-Device-ID")
	if deviceID == "" {
		log.Error().Msg("Missing X-Device-ID header")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-Device-ID header"})
		ctx.Abort()
		return
	}

	ctx.Next()
}

func (c *RunnerController) RegisterRoutes(router *gin.Engine) {
	api := router.Group("/api")
	{
		runners := api.Group("/runners")
		{
			runners.POST("", c.handleRunnerRegistration)
			runners.POST("/heartbeat", c.handleHeartbeat)

			tasks := runners.Group("/tasks")
			{
				tasks.GET("/available", c.handleAvailableTasks)
				tasks.POST("/:taskID/start", c.RequireDeviceID, c.handleTaskStart)
				tasks.POST("/:taskID/complete", c.handleTaskComplete)
				tasks.POST("/:taskID/result", c.RequireDeviceID, c.handleTaskResult)
			}
		}
	}
}

func (c *RunnerController) handleRunnerRegistration(ctx *gin.Context) {
	log := gologger.WithComponent("runner_controller")

	var req struct {
		WalletAddress string              `json:"wallet_address"`
		Status        models.RunnerStatus `json:"status"`
		Webhook       string              `json:"webhook"`
	}

	if err := ctx.BindJSON(&req); err != nil {
		log.Error().Err(err).Msg("Failed to parse runner registration request")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	deviceID := ctx.GetHeader("X-Device-ID")
	if deviceID == "" || req.WalletAddress == "" {
		log.Error().Msg("Missing required fields in runner registration")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "registered"})
}

func (c *RunnerController) handleHeartbeat(ctx *gin.Context) {
	log := gologger.WithComponent("runner_controller")

	deviceID := ctx.GetHeader("X-Device-ID")
	if deviceID == "" {
		log.Error().Msg("Missing X-Device-ID header")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Missing X-Device-ID header"})
		return
	}

	var msg struct {
		Type    string `json:"type"`
		Payload gin.H  `json:"payload"`
	}

	if err := ctx.BindJSON(&msg); err != nil {
		log.Error().Err(err).Msg("Failed to parse heartbeat message")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if msg.Type != "heartbeat" {
		log.Error().Str("type", msg.Type).Msg("Invalid message type")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message type"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// TODO: Implement the logic to handle available tasks
func (c *RunnerController) handleAvailableTasks(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, []interface{}{})
}

func (c *RunnerController) handleTaskStart(ctx *gin.Context) {
	log := gologger.WithComponent("runner_controller")

	taskID := ctx.Param("taskID")
	log.Debug().Str("task_id", taskID).Msg("Start task request received")

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *RunnerController) handleTaskComplete(ctx *gin.Context) {
	log := gologger.WithComponent("runner_controller")

	taskID := ctx.Param("taskID")
	log.Debug().Str("task_id", taskID).Msg("Complete task request received")

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *RunnerController) handleTaskResult(ctx *gin.Context) {
	log := gologger.WithComponent("runner_controller")

	taskID := ctx.Param("taskID")
	log.Debug().Str("task_id", taskID).Msg("Task result submission received")

	var result models.TaskResult
	if err := ctx.BindJSON(&result); err != nil {
		log.Error().Err(err).Msg("Failed to parse task result")
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}
