package keystoreutil

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

// GetKeystore creates and returns a keystore instance
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

// GetPrivateKey loads the private key from the keystore
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

// GetPrivateKeyHex returns the private key as a hex string
func GetPrivateKeyHex() (string, error) {
	privateKey, err := GetPrivateKey()
	if err != nil {
		return "", err
	}

	return common.Bytes2Hex(crypto.FromECDSA(privateKey)), nil
}

// SavePrivateKey saves a private key to the keystore
func SavePrivateKey(privateKeyHex string) error {
	ks, err := GetKeystore()
	if err != nil {
		return err
	}

	return ks.SavePrivateKey(privateKeyHex)
}
