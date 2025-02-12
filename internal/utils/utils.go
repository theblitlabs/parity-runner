package utils

import (
	"fmt"
	"math/big"
)

// formatEther converts wei (big.Int) to ether (string)
func FormatEther(wei *big.Int) string {
	ether := new(big.Float).SetInt(wei)
	ether.Quo(ether, new(big.Float).SetFloat64(1e18))
	return fmt.Sprintf("%.18f", ether)
}
