package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/virajbhartiya/parity-protocol/internal/models"
)

type MockTaskRepository struct {
	mock.Mock
}

func (m *MockTaskRepository) Create(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepository) Get(ctx context.Context, id string) (*models.Task, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskRepository) Update(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepository) List(ctx context.Context, limit, offset int) ([]*models.Task, error) {
	args := m.Called(ctx, limit, offset)
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) ListByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error) {
	args := m.Called(ctx, status)
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) GetAll(ctx context.Context) ([]models.Task, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockTaskRepository) GetTaskResult(ctx context.Context, taskID string) (*models.TaskResult, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskResult), args.Error(1)
}

func (m *MockTaskRepository) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}
