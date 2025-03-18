package nonce

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/theblitlabs/gologger"
)

// VerifyDrandNonce verifies that a nonce format is valid
func VerifyDrandNonce(nonce string) error {
	log := gologger.WithComponent("nonce.verify")

	if nonce == "" {
		return fmt.Errorf("empty nonce")
	}

	// Verify nonce is valid hex
	if _, err := hex.DecodeString(nonce); err != nil {
		// Check if it might be a fallback UUID-based nonce
		parts := strings.Split(nonce, "-")
		if len(parts) < 2 {
			return fmt.Errorf("invalid nonce format: not hex and not UUID-based")
		}

		timestamp := parts[0]
		if _, err := strconv.ParseInt(timestamp, 10, 64); err != nil {
			return fmt.Errorf("invalid nonce format: invalid timestamp in UUID-based nonce")
		}
	}

	log.Debug().
		Str("nonce", nonce).
		Msg("Nonce format verified")

	return nil
}
