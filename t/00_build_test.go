package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	bin := getBinary()
	os.Remove(bin)

	cgo := "0"
	if runtime.GOOS == "darwin" {
		cgo = "1"
	}

	runCmd(t, &cmd{
		Cmd:  "go",
		Args: []string{"build", "-buildvcs=false", "-o", bin}, // avoid: error obtaining VCS status: exit status 128
		Dir:  filepath.Join("..", "cmd", "snclient"),
		Env: map[string]string{
			"CGO_ENABLED": cgo,
		},
		ErrLike: []string{`.*`},
		Timeout: 5 * time.Minute,
	})

	require.FileExistsf(t, bin, "snclient binary must exist")
}
