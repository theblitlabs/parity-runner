package utils

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/crypto"
)

type KeyStore struct {
	PrivateKey string `json:"private_key"`
}

func FormatEther(wei *big.Int) string {
	ether := new(big.Float).SetInt(wei)
	ether.Quo(ether, new(big.Float).SetFloat64(1e18))
	return fmt.Sprintf("%.18f", ether)
}

func GetWalletAddress() (string, error) {
	keystoreDir := filepath.Join(os.Getenv("HOME"), ".parity")
	keystorePath := filepath.Join(keystoreDir, "keystore.json")

	data, err := os.ReadFile(keystorePath)
	if err != nil {
		return "", fmt.Errorf("failed to read keystore: %w", err)
	}

	var keystore KeyStore
	if err := json.Unmarshal(data, &keystore); err != nil {
		return "", fmt.Errorf("failed to parse keystore: %w", err)
	}

	privateKey := keystore.PrivateKey
	ecdsaKey, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return "", fmt.Errorf("invalid private key in keystore: %w", err)
	}

	address := crypto.PubkeyToAddress(ecdsaKey.PublicKey)
	return address.Hex(), nil
}
