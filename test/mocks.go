package test

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
)

// Mock implementations
type MockDockerExecutor struct {
	mock.Mock
}

func (m *MockDockerExecutor) ExecuteTask(ctx context.Context, task *models.Task) (*models.TaskResult, error) {
	args := m.Called(ctx, task)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskResult), args.Error(1)
}

type MockTaskClient struct {
	mock.Mock
}

func (m *MockTaskClient) GetAvailableTasks() ([]*models.Task, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskClient) StartTask(taskID string) error {
	args := m.Called(taskID)
	return args.Error(0)
}

func (m *MockTaskClient) CompleteTask(taskID string) error {
	args := m.Called(taskID)
	return args.Error(0)
}

func (m *MockTaskClient) SaveTaskResult(taskID string, result *models.TaskResult) error {
	args := m.Called(taskID, result)
	return args.Error(0)
}

type MockRewardClient struct {
	mock.Mock
}

func (m *MockRewardClient) DistributeRewards(result *models.TaskResult) error {
	args := m.Called(result)
	return args.Error(0)
}

type MockStakeWallet struct {
	mock.Mock
}

func (m *MockStakeWallet) GetStakeInfo(opts *bind.CallOpts, deviceID string) (stakewallet.StakeInfo, error) {
	args := m.Called(opts, deviceID)
	return args.Get(0).(stakewallet.StakeInfo), args.Error(1)
}

func (m *MockStakeWallet) TransferPayment(opts *bind.TransactOpts, creator string, runner string, amount *big.Int) error {
	args := m.Called(opts, creator, runner, amount)
	return args.Error(0)
}

type MockHandler struct {
	mock.Mock
}

func (m *MockHandler) HandleTask(task *models.Task) error {
	args := m.Called(task)
	return args.Error(0)
}
