package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jmoiron/sqlx"
	"github.com/virajbhartiya/parity-protocol/internal/api"
	"github.com/virajbhartiya/parity-protocol/internal/api/handlers"
	"github.com/virajbhartiya/parity-protocol/internal/config"
	"github.com/virajbhartiya/parity-protocol/internal/database/repositories"
	"github.com/virajbhartiya/parity-protocol/internal/services"
	"github.com/virajbhartiya/parity-protocol/pkg/database"
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
			log.Error().Msg("Unknown command. Use 'auth' or 'daemon'")
		}
	}

	log.Error().Msg("No command specified. Use 'parity auth' or 'parity daemon'")
}

func runServer() {
	// Existing server code here
	log := logger.Get()

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Error().Err(err).Msg("Failed to load config")
	}

	// Check if wallet is connected
	if err := helper.CheckWalletConnection(cfg); err != nil {
		switch {
		case err == helper.ErrInvalidInfuraKey:
			log.Error().Msg("Invalid Infura Project ID. Please update your config.yaml with a valid ID")
		case err == helper.ErrNoAuthToken:
			log.Error().Msg("Authentication required. Run 'parity auth --private-key YOUR_KEY' to connect your wallet")
		case err == helper.ErrInvalidAuthToken:
			log.Error().Msg("Invalid authentication token. Please re-authenticate with 'parity auth --private-key YOUR_KEY'")
		default:
			log.Error().Err(err).Msg("Failed to connect wallet")
		}
		os.Exit(1)
	}

	// Create database connection with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.Database.URL)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to database")
		os.Exit(1)
	}

	// Convert sql.DB to sqlx.DB
	dbx := sqlx.NewDb(db, "postgres")

	// Ping database to verify connection
	if err := db.PingContext(ctx); err != nil {
		log.Error().Err(err).Msg("Database connection check failed")
		os.Exit(1)
	}

	log.Info().Msg("Successfully connected to database")

	// Initialize database
	taskRepo := repositories.NewTaskRepository(dbx)
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
		log.Error().Err(err).Msg("Server failed to start")
	}
}

func runAuth() {
	log := logger.Get()

	// Create a new FlagSet for auth command
	authFlags := flag.NewFlagSet("auth", flag.ExitOnError)
	privateKeyFlag := authFlags.String("private-key", "", "Ethereum wallet private key")

	// Parse auth command flags
	if err := authFlags.Parse(os.Args[2:]); err != nil {
		log.Error().Err(err).Msg("Failed to parse flags")
	}

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

	// Save token to keystore instead of printing export command
	if err := keystore.SaveToken(token); err != nil {
		log.Error().Err(err).Msg("Failed to save authentication token")
		os.Exit(1)
	}

	fmt.Printf("\n✅ Authentication successful!\n\n")
	keystorePath, _ := keystore.GetKeystorePath()
	fmt.Printf("Token saved to: %s\n\n", keystorePath)

	keystorePath, _ = keystore.GetKeystorePath()
	log.Info().Str("path", keystorePath).Msg("Authentication successful. Token saved")
}
