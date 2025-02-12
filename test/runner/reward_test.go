package runner

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/theblitlabs/parity-protocol/internal/runner"
	"github.com/theblitlabs/parity-protocol/test"
)

func TestRewardClient(t *testing.T) {
	t.Run("distribute_rewards_success", func(t *testing.T) {
		client := runner.NewEthereumRewardClient(test.TestConfig)
		mockStakeWallet := &test.MockStakeWallet{}
		client.SetStakeWallet(mockStakeWallet)

		result := test.CreateTestResult()
		stakeInfo := test.CreateTestStakeInfo(true)

		mockStakeWallet.On("GetStakeInfo", mock.Anything, result.DeviceID).Return(stakeInfo, nil)
		mockStakeWallet.On("TransferPayment",
			mock.Anything,
			result.CreatorAddress,
			result.DeviceID,
			mock.MatchedBy(func(amount *big.Int) bool {
				expected := new(big.Float).Mul(
					big.NewFloat(1.5),
					new(big.Float).SetFloat64(1e18),
				)
				expectedInt, _ := expected.Int(nil)
				return amount.Cmp(expectedInt) == 0
			}),
		).Return(nil)

		err := client.DistributeRewards(result)
		assert.NoError(t, err)
		mockStakeWallet.AssertExpectations(t)
	})

	t.Run("distribute_rewards_no_stake", func(t *testing.T) {
		client := runner.NewEthereumRewardClient(test.TestConfig)
		mockStakeWallet := &test.MockStakeWallet{}
		client.SetStakeWallet(mockStakeWallet)

		result := test.CreateTestResult()
		stakeInfo := test.CreateTestStakeInfo(false)
		mockStakeWallet.On("GetStakeInfo", mock.Anything, result.DeviceID).Return(stakeInfo, nil)

		err := client.DistributeRewards(result)
		assert.NoError(t, err)
		mockStakeWallet.AssertNotCalled(t, "TransferPayment")
	})
}