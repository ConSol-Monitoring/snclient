package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCleanup(t *testing.T) {
	bin := getBinary()

	os.Remove(bin)
	require.NoFileExistsf(t, bin, "snclient binary must not exist anymore")
}
