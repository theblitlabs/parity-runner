package cli

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/spf13/cobra"
	"github.com/theblitlabs/deviceid"
	"github.com/theblitlabs/gologger"
	"github.com/theblitlabs/keystore"
	"github.com/theblitlabs/parity-protocol/internal/config"
	"github.com/theblitlabs/parity-protocol/internal/utils"
	"github.com/theblitlabs/parity-protocol/pkg/stakewallet"
	"github.com/theblitlabs/parity-protocol/pkg/wallet"
)

func RunStake() {
	var amount float64

	log := gologger.WithComponent("stake")
	log.Info().Msg("Starting staking process...")

	cmd := &cobra.Command{
		Use:   "stake",
		Short: "Stake tokens in the network",
		Run: func(cmd *cobra.Command, args []string) {
			log.Info().
				Float64("amount", amount).
				Msg("Processing stake request")
			executeStake(amount)
		},
	}

	cmd.Flags().Float64VarP(&amount, "amount", "a", 1.0, "Amount of PRTY tokens to stake")
	if err := cmd.MarkFlagRequired("amount"); err != nil {
		log.Error().Err(err).Msg("Failed to mark amount flag as required")
	}

	if err := cmd.Execute(); err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to execute stake command - please check your input")
	}
}

func executeStake(amount float64) {
	log := gologger.WithComponent("stake")

	// Load configuration
	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to load configuration - please ensure config.yaml exists")
		return
	}

	// Get private key from keystore
	ks, err := keystore.NewKeystore(keystore.Config{})
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to create keystore")
		return
	}

	privateKey, err := ks.LoadPrivateKey()
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("No private key found - please authenticate first using 'parity auth'")
		return
	}

	// Create Ethereum client
	client, err := wallet.NewClientWithKey(
		cfg.Ethereum.RPC,
		big.NewInt(cfg.Ethereum.ChainID),
		common.Bytes2Hex(crypto.FromECDSA(privateKey)),
	)
	if err != nil {
		log.Fatal().
			Err(err).
			Str("rpc_endpoint", cfg.Ethereum.RPC).
			Int64("chain_id", cfg.Ethereum.ChainID).
			Msg("Failed to connect to blockchain - please check your network connection")
		return
	}

	// Verify device ID
	deviceIDManager := deviceid.NewManager(deviceid.Config{})
	deviceID, err := deviceIDManager.VerifyDeviceID()
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to verify device - please ensure you have a valid device ID")
		return
	}

	log.Info().
		Str("device_id", deviceID).
		Str("wallet", client.Address().Hex()).
		Msg("Device verified successfully")

	tokenAddr := common.HexToAddress(cfg.Ethereum.TokenAddress)
	stakeWalletAddr := common.HexToAddress(cfg.Ethereum.StakeWalletAddress)

	// Check token balance
	balance, err := client.GetERC20Balance(tokenAddr, client.Address())
	if err != nil {
		log.Fatal().
			Err(err).
			Str("token_address", tokenAddr.Hex()).
			Str("wallet", client.Address().Hex()).
			Msg("Failed to check token balance - please try again")
		return
	}

	amountToStake := amountWei(amount)
	if balance.Cmp(amountToStake) < 0 {
		log.Fatal().
			Str("current_balance", utils.FormatEther(balance)+" PRTY").
			Str("required_amount", utils.FormatEther(amountToStake)+" PRTY").
			Msg("Insufficient token balance - please ensure you have enough PRTY tokens")
		return
	}

	log.Info().
		Str("balance", utils.FormatEther(balance)+" PRTY").
		Str("token_address", tokenAddr.Hex()).
		Msg("Current token balance verified")

	// Check allowance
	allowance, err := client.GetAllowance(tokenAddr, client.Address(), stakeWalletAddr)
	if err != nil {
		log.Fatal().
			Err(err).
			Str("token_address", tokenAddr.Hex()).
			Str("stake_wallet", stakeWalletAddr.Hex()).
			Msg("Failed to check token allowance - please try again")
		return
	}

	// Approve token spending if needed
	if allowance.Cmp(amountToStake) < 0 {
		log.Info().
			Str("amount", utils.FormatEther(amountToStake)+" PRTY").
			Msg("Approving token spending...")

		txOpts, err := client.GetTransactOpts()
		if err != nil {
			log.Fatal().
				Err(err).
				Msg("Failed to prepare transaction - please try again")
			return
		}

		tx, err := client.ApproveToken(txOpts, tokenAddr, stakeWalletAddr, amountToStake)
		if err != nil {
			log.Fatal().
				Err(err).
				Str("amount", utils.FormatEther(amountToStake)+" PRTY").
				Msg("Failed to approve token spending - please try again")
			return
		}

		log.Info().
			Str("tx_hash", tx.Hash().Hex()).
			Str("amount", utils.FormatEther(amountToStake)+" PRTY").
			Msg("Token approval submitted - waiting for confirmation...")

		// Wait for approval confirmation
		receipt, err := bind.WaitMined(context.Background(), client, tx)
		if err != nil {
			log.Fatal().
				Err(err).
				Str("tx_hash", tx.Hash().Hex()).
				Msg("Failed to confirm token approval - please check the transaction status")
			return
		}

		if receipt.Status == 0 {
			log.Fatal().
				Str("tx_hash", tx.Hash().Hex()).
				Msg("Token approval failed - please check the transaction status")
			return
		}

		log.Info().
			Str("tx_hash", tx.Hash().Hex()).
			Msg("Token approval confirmed successfully")

		// Small delay to ensure approval is propagated
		time.Sleep(5 * time.Second)
	}

	// Create stake wallet contract instance
	stakeWallet, err := stakewallet.NewStakeWallet(stakeWalletAddr, client)
	if err != nil {
		log.Fatal().
			Err(err).
			Str("stake_wallet", stakeWalletAddr.Hex()).
			Msg("Failed to connect to stake contract - please try again")
		return
	}

	// Prepare staking transaction
	txOpts, err := client.GetTransactOpts()
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to prepare staking transaction - please try again")
		return
	}

	log.Info().
		Str("amount", utils.FormatEther(amountToStake)+" PRTY").
		Str("device_id", deviceID).
		Msg("Submitting stake transaction...")

	// Execute stake transaction
	tx, err := stakeWallet.Stake(txOpts, amountToStake, deviceID, client.Address())
	if err != nil {
		log.Fatal().
			Err(err).
			Str("amount", utils.FormatEther(amountToStake)+" PRTY").
			Str("device_id", deviceID).
			Msg("Failed to submit stake transaction - please try again")
		return
	}

	log.Info().
		Str("tx_hash", tx.Hash().Hex()).
		Str("amount", utils.FormatEther(amountToStake)+" PRTY").
		Str("device_id", deviceID).
		Str("wallet", client.Address().Hex()).
		Msg("Stake transaction submitted - waiting for confirmation...")

	// Wait for stake transaction confirmation
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		log.Error().
			Err(err).
			Str("tx_hash", tx.Hash().Hex()).
			Msg("Failed to confirm stake transaction - please check the transaction status")
		return
	}

	if receipt.Status == 1 {
		log.Info().
			Str("tx_hash", tx.Hash().Hex()).
			Str("amount", utils.FormatEther(amountToStake)+" PRTY").
			Str("device_id", deviceID).
			Str("wallet", client.Address().Hex()).
			Uint64("block_number", receipt.BlockNumber.Uint64()).
			Msg("Stake transaction confirmed successfully! Your device is now registered and ready to process tasks.")
	} else {
		log.Error().
			Str("tx_hash", tx.Hash().Hex()).
			Str("amount", utils.FormatEther(amountToStake)+" PRTY").
			Msg("Stake transaction failed - please check the transaction status")
	}
}

func amountWei(amount float64) *big.Int {
	amountWei := new(big.Float).Mul(
		big.NewFloat(amount),
		new(big.Float).SetFloat64(1e18),
	)
	wei, _ := amountWei.Int(nil)
	return wei
}
