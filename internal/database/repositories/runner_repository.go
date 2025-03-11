package repositories

import (
	"context"
	"errors"

	"github.com/theblitlabs/parity-protocol/internal/models"
	"gorm.io/gorm"
)

var (
	ErrRunnerNotFound = errors.New("runner not found")
)

type RunnerRepository struct {
	db *gorm.DB
}

func NewRunnerRepository(db *gorm.DB) *RunnerRepository {
	return &RunnerRepository{db: db}
}

func (r *RunnerRepository) Create(ctx context.Context, runner *models.Runner) error {
	return r.db.Create(runner).Error
}

func (r *RunnerRepository) safe_add(ctx context.Context, runner *models.Runner) error {
	var existingRunner models.Runner
	var err error
	if err = r.db.Where("device_id = ?", runner.DeviceID).First(&existingRunner).Error; err == nil {
		// Update existing runner fields
		existingRunner.Address = runner.Address
		existingRunner.Status = runner.Status
		existingRunner.TaskID = runner.TaskID
		existingRunner.Webhook = runner.Webhook
		err = r.db.Save(&existingRunner).Error
		if err != nil {
			return err
		}
		return nil
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create new runner
		err = r.Create(ctx, runner)
		if err != nil {
			return err
		}
		return nil
	}
	return err
}

func (r *RunnerRepository) GetByID(ctx context.Context, id string) (*models.Runner, error) {
	var runner models.Runner
	if err := r.db.Where("id = ?", id).First(&runner).Error; err != nil {
		return nil, err
	}
	return &runner, nil
}