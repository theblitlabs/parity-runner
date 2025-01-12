package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/virajbhartiya/parity-protocol/internal/api"
	"github.com/virajbhartiya/parity-protocol/internal/api/handlers"
	"github.com/virajbhartiya/parity-protocol/internal/config"
	"github.com/virajbhartiya/parity-protocol/internal/database"
	"github.com/virajbhartiya/parity-protocol/internal/database/repositories"
	"github.com/virajbhartiya/parity-protocol/internal/services"
	"github.com/virajbhartiya/parity-protocol/pkg/helper"
	"github.com/virajbhartiya/parity-protocol/pkg/keystore"
	"github.com/virajbhartiya/parity-protocol/pkg/logger"
	"github.com/virajbhartiya/parity-protocol/pkg/wallet"
)

func main() {
	logger.Init()
	log := logger.Get()

	// Check command
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			runAuth()
			return
		case "daemon":
			runServer()
			return
		default:
			log.Fatal().Msg("Unknown command. Use 'auth' or 'daemon'")
		}
	}

	log.Fatal().Msg("No command specified. Use 'parity auth' or 'parity daemon'")
}

func runServer() {
	// Existing server code here
	log := logger.Get()

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Check if wallet is connected
	if err := helper.CheckWalletConnection(cfg); err != nil {
		switch {
		case err == helper.ErrInvalidInfuraKey:
			log.Fatal().Msg("Invalid Infura Project ID. Please update your config.yaml with a valid ID")
		case err == helper.ErrNoAuthToken:
			log.Fatal().Msg("Authentication required. Run 'parity auth --private-key YOUR_KEY' to connect your wallet")
		case err == helper.ErrInvalidAuthToken:
			log.Fatal().Msg("Invalid authentication token. Please re-authenticate with 'parity auth --private-key YOUR_KEY'")
		default:
			log.Fatal().Err(err).Msg("Failed to connect wallet")
		}
		os.Exit(1)
	}

	// Initialize database
	db, err := database.NewDB(&cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Initialize services
	taskRepo := repositories.NewTaskRepository(db)
	taskService := services.NewTaskService(taskRepo)
	taskHandler := handlers.NewTaskHandler(taskService)

	// Initialize API handlers and start server
	router := api.NewRouter(
		taskHandler,
		cfg.Server.Endpoint,
	)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler: router,
	}

	log.Info().Msgf("Server starting on %s", fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port))

	if err := server.ListenAndServe(); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}

func runAuth() {
	log := logger.Get()

	// Create a new FlagSet for auth command
	authFlags := flag.NewFlagSet("auth", flag.ExitOnError)
	privateKeyFlag := authFlags.String("private-key", "", "Ethereum wallet private key")

	// Parse auth command flags
	if err := authFlags.Parse(os.Args[2:]); err != nil {
		log.Fatal().Err(err).Msg("Failed to parse flags")
	}

	if *privateKeyFlag == "" {
		log.Fatal().Msg("Private key is required. Use --private-key flag")
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Initialize wallet
	privateKey, err := crypto.HexToECDSA(*privateKeyFlag)
	if err != nil {
		log.Fatal().Msg("Invalid private key format")
		os.Exit(1)
	}

	// Create wallet client
	client, err := wallet.NewClient(cfg.Ethereum.RPC, cfg.Ethereum.ChainID)
	if err != nil {
		if err.Error() == "401 Unauthorized: invalid project id" {
			log.Fatal().Msg("Invalid Infura Project ID. Please update your config.yaml with a valid ID")
		} else {
			log.Fatal().Err(err).Msg("Failed to connect to Ethereum network")
		}
		os.Exit(1)
	}

	// Get wallet address
	address := crypto.PubkeyToAddress(privateKey.PublicKey)

	// Check ERC20 token balance
	tokenContract := common.HexToAddress(cfg.Ethereum.TokenAddress)
	balance, err := client.GetERC20Balance(tokenContract, address)
	if err != nil {
		log.Fatal().Msg("Failed to connect to token contract. Please check your network connection")
		os.Exit(1)
	}

	if balance.Sign() <= 0 {
		log.Fatal().Msg("No tokens found in wallet. Please ensure you have Parity tokens in your wallet")
		os.Exit(1)
	}

	// Generate authentication token
	token, err := wallet.GenerateToken(address.Hex(), privateKey)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to generate authentication token")
		os.Exit(1)
	}

	// Save token to keystore instead of printing export command
	if err := keystore.SaveToken(token); err != nil {
		log.Fatal().Err(err).Msg("Failed to save authentication token")
		os.Exit(1)
	}

	fmt.Printf("\n✅ Authentication successful!\n\n")
	keystorePath, _ := keystore.GetKeystorePath()
	fmt.Printf("Token saved to: %s\n\n", keystorePath)
}

// Add this helper function
func isAuthCommand() bool {
	if len(os.Args) > 1 {
		return os.Args[1] == "auth"
	}
	return false
}
