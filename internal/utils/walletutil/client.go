package walletutil

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	walletsdk "github.com/theblitlabs/go-wallet-sdk"
	"github.com/theblitlabs/parity-runner/internal/core/config"
	"github.com/theblitlabs/parity-runner/internal/utils/keystoreutil"
)

var (
	clientInstance *walletsdk.Client
	clientMutex    sync.Mutex
)

func NewClient(cfg *config.Config) (*walletsdk.Client, error) {
	if clientInstance != nil {
		return clientInstance, nil
	}

	clientMutex.Lock()
	defer clientMutex.Unlock()

	if clientInstance != nil {
		return clientInstance, nil
	}

	privateKeyHex, err := keystoreutil.GetPrivateKeyHex()
	if err != nil {
		return nil, fmt.Errorf("failed to get private key: %w", err)
	}

	clientConfig := walletsdk.ClientConfig{
		RPCURL:       cfg.Ethereum.RPC,
		ChainID:      cfg.Ethereum.ChainID,
		PrivateKey:   privateKeyHex,
		TokenAddress: common.HexToAddress(cfg.Ethereum.TokenAddress),
	}

	if cfg.Ethereum.StakeWalletAddress != "" {
		clientConfig.StakeAddress = common.HexToAddress(cfg.Ethereum.StakeWalletAddress)
	}

	client, err := walletsdk.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet client: %w", err)
	}

	clientInstance = client
	return client, nil
}

func ResetClient() {
	clientMutex.Lock()
	defer clientMutex.Unlock()
	clientInstance = nil
}

func GetClientWithConfig(config walletsdk.ClientConfig) (*walletsdk.Client, error) {
	client, err := walletsdk.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet client: %w", err)
	}
	return client, nil
}

func GetClientWithPrivateKey(cfg *config.Config, privateKeyHex string) (*walletsdk.Client, error) {
	clientConfig := walletsdk.ClientConfig{
		RPCURL:       cfg.Ethereum.RPC,
		ChainID:      cfg.Ethereum.ChainID,
		PrivateKey:   privateKeyHex,
		TokenAddress: common.HexToAddress(cfg.Ethereum.TokenAddress),
	}

	if cfg.Ethereum.StakeWalletAddress != "" {
		clientConfig.StakeAddress = common.HexToAddress(cfg.Ethereum.StakeWalletAddress)
	}

	return GetClientWithConfig(clientConfig)
}
