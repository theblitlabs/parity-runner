package nonce

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/theblitlabs/gologger"
)

func VerifyDrandNonce(nonce string) error {
	log := gologger.WithComponent("nonce.verify")

	if nonce == "" {
		return fmt.Errorf("empty nonce")
	}

	if _, err := hex.DecodeString(nonce); err != nil {

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
