package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"github.com/virajbhartiya/parity-protocol/internal/models"
)

type MockTaskRepository struct {
	mock.Mock
}

// Add other existing mock methods...

func (m *MockTaskRepository) GetAll(ctx context.Context) ([]models.Task, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Task), args.Error(1)
}
