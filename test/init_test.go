package test

import (
	"testing"
)

func init() {
	DisableLogging()
}

func TestMain(m *testing.M) {
	// Run tests
	m.Run()
}
