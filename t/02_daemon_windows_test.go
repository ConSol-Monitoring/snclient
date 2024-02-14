package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDaemonRequestsWindows(t *testing.T) {
	bin := getBinary()
	require.FileExistsf(t, bin, "snclient binary must exist")

	writeFile(t, `snclient.ini`, localDaemonINI)

	startBackgroundDaemon(t)

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", localDaemonPort)

	runCmd(t, &cmd{
		Cmd:  "curl",
		Args: []string{"-s", "-k", "--header", "password:" + localDaemonPassword, baseURL + "/query/check_echo_win?%20"},
		Like: []string{`"check_echo_win"`, `"OK"`},
		Exit: 0,
	})

	stopBackgroundDaemon(t)
	os.Remove("snclient.ini")
	os.Remove("test.crt")
	os.Remove("test.key")
}
