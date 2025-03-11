package main
// TODO: change the package to services and remove main function

import (
	"context"
	"fmt"
	"time"

	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/database/repositories"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/database"

)

type RunnerService struct {
	repo repositories.RunnerRepository
}

func NewRunnerService(repo repositories.RunnerRepository) *RunnerService {
	return &RunnerService{repo: repo}
}

func (s *RunnerService) CreateRunner(ctx context.Context, runner *models.Runner) error {
	return s.repo.Create(ctx, runner)
}



func main() {
	cfg, err := config.LoadConfig("./config/config.yaml")
	if err != nil {
		fmt.Println(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Connect(ctx, cfg.Database.URL)
	if err != nil {
		fmt.Println(err)
	}
	runnerRepo := repositories.NewRunnerRepository(db)
	runnerService := NewRunnerService(*runnerRepo)

	runner := &models.Runner{
		DeviceID: "test",
		Address:  "0x1234567890123456789012345678901234567890",
		Status:   models.RunnerStatusOnline,
		Webhook:  "http://localhost:8080/webhook",
	}
	runnerService.CreateRunner(ctx, runner)

}