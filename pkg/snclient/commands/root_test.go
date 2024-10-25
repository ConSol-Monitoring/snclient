package commands

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmdVersion(t *testing.T) {
	out, err := runCommand(t, []string{"-V"})
	require.NoError(t, err, "command runs without error")
	assert.Contains(t, out, "SNClient+ v", "output matches")
}

func TestCmdHelp(t *testing.T) {
	out, err := runCommand(t, []string{"-h"})
	require.NoError(t, err, "command runs without error")
	assert.Contains(t, out, "Usage:", "output matches")
}

// runCommand runs cmd and returns output / error
func runCommand(t *testing.T, args []string) (output string, err error) {
	t.Helper()

	outFile, _ := os.CreateTemp("", "snclient-test")
	sout := os.Stdout
	serr := os.Stderr
	os.Stdout = outFile
	os.Stderr = outFile
	defer func() {
		os.Stdout = sout
		os.Stderr = serr
	}()

	rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		err = f.Value.Set(f.DefValue)
		require.NoError(t, err)
	})
	rootCmd.SetArgs(args)
	err = Execute()
	outFile.Close()
	outputBytes, err2 := os.ReadFile(outFile.Name())

	require.NoErrorf(t, err, "command errored, output:\n%s", string(outputBytes))

	return string(outputBytes), err2
}
