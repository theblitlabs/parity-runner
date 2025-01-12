package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/virajbhartiya/parity-protocol/internal/config"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
	"github.com/virajbhartiya/parity-protocol/pkg/wallet"
)

func main() {
	logger.Init()
	log := logger.Get()

	// Parse command line flags
	privateKeyFlag := flag.String("private-key", "", "Ethereum wallet private key")
	flag.Parse()

	if *privateKeyFlag == "" {
		log.Error().Msg("Private key is required. Use --private-key flag")
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Error().Err(err).Msg("Failed to load config")
	}

	// Initialize wallet
	privateKey, err := crypto.HexToECDSA(*privateKeyFlag)
	if err != nil {
		log.Error().Msg("Invalid private key format")
		os.Exit(1)
	}

	// Create wallet client
	client, err := wallet.NewClient(cfg.Ethereum.RPC, cfg.Ethereum.ChainID)
	if err != nil {
		if err.Error() == "401 Unauthorized: invalid project id" {
			log.Error().Msg("Invalid Infura Project ID. Please update your config.yaml with a valid ID")
		} else {
			log.Error().Err(err).Msg("Failed to connect to Ethereum network")
		}
		os.Exit(1)
	}

	// Get wallet address
	address := crypto.PubkeyToAddress(privateKey.PublicKey)

	// Check ERC20 token balance
	tokenContract := common.HexToAddress(cfg.Ethereum.TokenAddress)
	balance, err := client.GetERC20Balance(tokenContract, address)
	if err != nil {
		log.Error().Msg("Failed to connect to token contract. Please check your network connection")
		os.Exit(1)
	}

	if balance.Sign() <= 0 {
		log.Error().Msg("No tokens found in wallet. Please ensure you have Parity tokens in your wallet")
		os.Exit(1)
	}

	// Generate authentication token
	token, err := wallet.GenerateToken(address.Hex(), privateKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate authentication token")
		os.Exit(1)
	}

	fmt.Printf("\n✅ Authentication successful!\n\n")
	fmt.Printf("Token: %s\n\n", token)
	fmt.Printf("To use this token, set the environment variable:\n")
	fmt.Printf("export PARITY_AUTH_TOKEN='%s'\n\n", token)
}
