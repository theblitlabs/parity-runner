package utils

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/theblitlabs/keystore"
)

const (
	KeystoreDirName  = ".parity"
	KeystoreFileName = "keystore.json"
)

func GetKeystore() (*keystore.Store, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	ks, err := keystore.NewKeystore(keystore.Config{
		DirPath:  filepath.Join(homeDir, KeystoreDirName),
		FileName: KeystoreFileName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create keystore: %w", err)
	}

	return ks, nil
}

func GetPrivateKey() (*ecdsa.PrivateKey, error) {
	ks, err := GetKeystore()
	if err != nil {
		return nil, err
	}

	privateKey, err := ks.LoadPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("no private key found - please authenticate first using 'parity auth': %w", err)
	}

	return privateKey, nil
}

func GetPrivateKeyHex() (string, error) {
	privateKey, err := GetPrivateKey()
	if err != nil {
		return "", err
	}

	return common.Bytes2Hex(crypto.FromECDSA(privateKey)), nil
}

func SavePrivateKey(privateKeyHex string) error {
	ks, err := GetKeystore()
	if err != nil {
		return err
	}

	return ks.SavePrivateKey(privateKeyHex)
}
