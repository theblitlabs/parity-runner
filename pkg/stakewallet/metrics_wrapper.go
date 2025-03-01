package stakewallet

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/theblitlabs/parity-protocol/internal/telemetry"
)

// MetricsStakeWallet wraps a StakeWallet and adds metrics
type MetricsStakeWallet struct {
	sw StakeWallet
}

// NewMetricsStakeWallet creates a new metrics-enabled stake wallet
func NewMetricsStakeWallet(sw StakeWallet) *MetricsStakeWallet {
	return &MetricsStakeWallet{sw: sw}
}

// GetBalanceByDeviceID implements StakeWallet interface with metrics
func (m *MetricsStakeWallet) GetBalanceByDeviceID(opts *bind.CallOpts, deviceID string) (*big.Int, error) {
	balance, err := m.sw.GetBalanceByDeviceID(opts, deviceID)
	if err != nil {
		telemetry.RecordStakeOperation("get_balance", "error")
		return nil, err
	}

	telemetry.RecordStakeOperation("get_balance", "success")
	return balance, nil
}

// GetStakeInfo implements StakeWallet interface with metrics
func (m *MetricsStakeWallet) GetStakeInfo(opts *bind.CallOpts, deviceID string) (StakeInfo, error) {
	info, err := m.sw.GetStakeInfo(opts, deviceID)
	if err != nil {
		telemetry.RecordStakeOperation("get_info", "error")
		return info, err
	}

	telemetry.RecordStakeOperation("get_info", "success")
	return info, nil
}

// Owner implements StakeWallet interface with metrics
func (m *MetricsStakeWallet) Owner(opts *bind.CallOpts) (common.Address, error) {
	owner, err := m.sw.Owner(opts)
	if err != nil {
		telemetry.RecordStakeOperation("get_owner", "error")
		return owner, err
	}

	telemetry.RecordStakeOperation("get_owner", "success")
	return owner, nil
}

// Token implements StakeWallet interface with metrics
func (m *MetricsStakeWallet) Token(opts *bind.CallOpts) (common.Address, error) {
	token, err := m.sw.Token(opts)
	if err != nil {
		telemetry.RecordStakeOperation("get_token", "error")
		return token, err
	}

	telemetry.RecordStakeOperation("get_token", "success")
	return token, nil
}

// Stake implements StakeWallet interface with metrics
func (m *MetricsStakeWallet) Stake(opts *bind.TransactOpts, amount *big.Int, deviceID string, walletAddr common.Address) (*types.Transaction, error) {
	tx, err := m.sw.Stake(opts, amount, deviceID, walletAddr)
	if err != nil {
		telemetry.RecordStakeOperation("stake", "error")
		return nil, err
	}

	telemetry.RecordStakeOperation("stake", "success")
	return tx, nil
}

// TransferPayment implements StakeWallet interface with metrics
func (m *MetricsStakeWallet) TransferPayment(opts *bind.TransactOpts, creatorDeviceID string, solverDeviceID string, amount *big.Int) (*types.Transaction, error) {
	tx, err := m.sw.TransferPayment(opts, creatorDeviceID, solverDeviceID, amount)
	if err != nil {
		telemetry.RecordStakeOperation("transfer", "error")
		return nil, err
	}

	telemetry.RecordStakeOperation("transfer", "success")
	return tx, nil
}
