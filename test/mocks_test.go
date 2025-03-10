package test

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/theblitlabs/parity-protocol/internal/models"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
)

func TestMockTaskRepository(t *testing.T) {
	mockRepo := new(MockTaskRepository)
	ctx := context.Background()
	task := CreateTestTask()

	t.Run("List", func(t *testing.T) {
		expectedTasks := []*models.Task{task}
		mockRepo.On("List", ctx, 0, 10).Return(expectedTasks, nil)

		tasks, err := mockRepo.List(ctx, 0, 10)
		assert.NoError(t, err)
		assert.Equal(t, expectedTasks, tasks)
		mockRepo.AssertExpectations(t)
	})

	t.Run("GetAll", func(t *testing.T) {
		expectedTasks := []models.Task{*task}
		mockRepo.On("GetAll", ctx).Return(expectedTasks, nil)

		tasks, err := mockRepo.GetAll(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expectedTasks, tasks)
		mockRepo.AssertExpectations(t)
	})

	t.Run("ListAvailable", func(t *testing.T) {
		expectedTasks := []*models.Task{task}
		mockRepo.On("ListAvailable", ctx).Return(expectedTasks, nil)

		tasks, err := mockRepo.ListAvailable(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expectedTasks, tasks)
		mockRepo.AssertExpectations(t)
	})

	t.Run("GetTaskResult", func(t *testing.T) {
		result := CreateTestResult()
		mockRepo.On("GetTaskResult", ctx, task.ID).Return(result, nil)

		gotResult, err := mockRepo.GetTaskResult(ctx, task.ID)
		assert.NoError(t, err)
		assert.Equal(t, result, gotResult)
		mockRepo.AssertExpectations(t)
	})

	t.Run("SaveResult", func(t *testing.T) {
		result := CreateTestResult()
		mockRepo.On("SaveResult", ctx, result).Return(nil)

		err := mockRepo.SaveResult(ctx, result)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("SaveTaskResult", func(t *testing.T) {
		result := CreateTestResult()
		mockRepo.On("SaveTaskResult", ctx, result).Return(nil)

		err := mockRepo.SaveTaskResult(ctx, result)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestMockIPFSClient(t *testing.T) {
	mockClient := new(MockIPFSClient)
	data := []byte("test data")
	cid := "QmTest123"

	t.Run("StoreJSON", func(t *testing.T) {
		mockClient.On("StoreJSON", data).Return(cid, nil)

		gotCID, err := mockClient.StoreJSON(data)
		assert.NoError(t, err)
		assert.Equal(t, cid, gotCID)
		mockClient.AssertExpectations(t)
	})

	t.Run("StoreData", func(t *testing.T) {
		mockClient.On("StoreData", data).Return(cid, nil)

		gotCID, err := mockClient.StoreData(data)
		assert.NoError(t, err)
		assert.Equal(t, cid, gotCID)
		mockClient.AssertExpectations(t)
	})

	t.Run("GetData", func(t *testing.T) {
		mockClient.On("GetData", cid).Return(data, nil)

		gotData, err := mockClient.GetData(cid)
		assert.NoError(t, err)
		assert.Equal(t, data, gotData)
		mockClient.AssertExpectations(t)
	})

	t.Run("GetJSON", func(t *testing.T) {
		var target interface{}
		mockClient.On("GetJSON", cid, &target).Return(nil)

		err := mockClient.GetJSON(cid, &target)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})
}

func TestMockDockerExecutor(t *testing.T) {
	mockExecutor := new(MockDockerExecutor)
	task := CreateTestTask()
	result := CreateTestResult()

	t.Run("ExecuteTask", func(t *testing.T) {
		mockExecutor.On("ExecuteTask", context.Background(), task).Return(result, nil)

		gotResult, err := mockExecutor.ExecuteTask(context.Background(), task)
		assert.NoError(t, err)
		assert.Equal(t, result, gotResult)
		mockExecutor.AssertExpectations(t)
	})
}

func TestMockTaskClient(t *testing.T) {
	mockClient := new(MockTaskClient)
	taskID := uuid.New().String()
	result := CreateTestResult()

	t.Run("GetAvailableTasks", func(t *testing.T) {
		tasks := []*models.Task{CreateTestTask()}
		mockClient.On("GetAvailableTasks").Return(tasks, nil)

		gotTasks, err := mockClient.GetAvailableTasks()
		assert.NoError(t, err)
		assert.Equal(t, tasks, gotTasks)
		mockClient.AssertExpectations(t)
	})

	t.Run("StartTask", func(t *testing.T) {
		mockClient.On("StartTask", taskID).Return(nil)

		err := mockClient.StartTask(taskID)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("CompleteTask", func(t *testing.T) {
		mockClient.On("CompleteTask", taskID).Return(nil)

		err := mockClient.CompleteTask(taskID)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})

	t.Run("SaveTaskResult", func(t *testing.T) {
		mockClient.On("SaveTaskResult", taskID, result).Return(nil)

		err := mockClient.SaveTaskResult(taskID, result)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})
}

func TestMockRewardClient(t *testing.T) {
	mockClient := new(MockRewardClient)
	result := CreateTestResult()

	t.Run("DistributeRewards", func(t *testing.T) {
		mockClient.On("DistributeRewards", result).Return(nil)

		err := mockClient.DistributeRewards(result)
		assert.NoError(t, err)
		mockClient.AssertExpectations(t)
	})
}

func TestMockStakeWallet(t *testing.T) {
	mockWallet := new(MockStakeWallet)
	opts := &bind.CallOpts{}
	txOpts := &bind.TransactOpts{}
	deviceID := "test-device"
	amount := big.NewInt(1000)

	t.Run("GetStakeInfo", func(t *testing.T) {
		expectedInfo := stakewallet.StakeInfo{
			Exists:   true,
			DeviceID: deviceID,
			Amount:   big.NewInt(1000),
		}
		mockWallet.On("GetStakeInfo", opts, deviceID).Return(expectedInfo, nil)

		info, err := mockWallet.GetStakeInfo(opts, deviceID)
		assert.NoError(t, err)
		assert.Equal(t, deviceID, info.DeviceID)
		assert.Equal(t, expectedInfo.Amount, info.Amount)
		mockWallet.AssertExpectations(t)
	})

	t.Run("TransferPayment", func(t *testing.T) {
		creatorDeviceID := "creator-device"
		solverDeviceID := "solver-device"
		mockWallet.On("TransferPayment", txOpts, creatorDeviceID, solverDeviceID, amount).Return(nil)

		err := mockWallet.TransferPayment(txOpts, creatorDeviceID, solverDeviceID, amount)
		assert.NoError(t, err)
		mockWallet.AssertExpectations(t)
	})
}

func TestMockHandler(t *testing.T) {
	mockHandler := new(MockHandler)
	task := CreateTestTask()

	t.Run("HandleTask", func(t *testing.T) {
		mockHandler.On("HandleTask", task).Return(nil)

		err := mockHandler.HandleTask(task)
		assert.NoError(t, err)
		mockHandler.AssertExpectations(t)
	})
}
