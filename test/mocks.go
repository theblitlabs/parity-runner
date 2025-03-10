package test

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/ipfs"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
)

// Mock implementations
type MockTaskRepository struct {
	mock.Mock
}

func (m *MockTaskRepository) Create(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepository) Get(ctx context.Context, id uuid.UUID) (*models.Task, error) {
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

func (m *MockTaskRepository) List(ctx context.Context, offset int, limit int) ([]*models.Task, error) {
	args := m.Called(ctx, offset, limit)
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) GetAll(ctx context.Context) ([]models.Task, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockTaskRepository) ListAvailable(ctx context.Context) ([]*models.Task, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) GetTaskResult(ctx context.Context, id uuid.UUID) (*models.TaskResult, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.TaskResult), args.Error(1)
}

func (m *MockTaskRepository) SaveResult(ctx context.Context, result *models.TaskResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

func (m *MockTaskRepository) ListByStatus(ctx context.Context, status models.TaskStatus) ([]*models.Task, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Task), args.Error(1)
}

func (m *MockTaskRepository) SaveTaskResult(ctx context.Context, result *models.TaskResult) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

type MockIPFSClient struct {
	mock.Mock
	ipfs.Client
}

func (m *MockIPFSClient) StoreJSON(data interface{}) (string, error) {
	args := m.Called(data)
	return args.String(0), args.Error(1)
}

func (m *MockIPFSClient) StoreData(data []byte) (string, error) {
	args := m.Called(data)
	return args.String(0), args.Error(1)
}

func (m *MockIPFSClient) GetData(cid string) ([]byte, error) {
	args := m.Called(cid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockIPFSClient) GetJSON(cid string, target interface{}) error {
	args := m.Called(cid, target)
	return args.Error(0)
}

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
