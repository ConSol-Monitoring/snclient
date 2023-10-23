package cmd

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCmdVersion(t *testing.T) {
	out, err := RunCommand(t, rootCmd, []string{"-V"})
	require.NoError(t, err, "command runs without error")
	assert.Contains(t, out, "SNClient+ v", "output matches")
}

func TestCmdHelp(t *testing.T) {
	out, err := RunCommand(t, rootCmd, []string{"-h"})
	require.NoError(t, err, "command runs without error")
	assert.Contains(t, out, "Usage:", "output matches")
}

// RunCommand runs cmd and returns output / error
func RunCommand(t *testing.T, cmd *cobra.Command, args []string) (output string, err error) {
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

	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		f.Value.Set(f.DefValue)
	})
	cmd.SetArgs(args)
	err = cmd.Execute()

	outFile.Close()

	outputBytes, err := os.ReadFile(outFile.Name())
	return string(outputBytes), err
}
