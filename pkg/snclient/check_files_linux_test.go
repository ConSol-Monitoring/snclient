package snclient

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckFilesPerfdataWhitespaceNames(t *testing.T) {
	testDir, _ := os.Getwd()
	scriptsDir := filepath.Join(testDir, "t", "scripts")
	scriptName := "check_files_perfdata_generate_whitespace_files"
	scriptFilename := scriptName

	switch runtime.GOOS {
	case "linux":
		scriptFilename += ".sh"
	default:
		t.Skipf("Test is not intended to be run on %s", runtime.GOOS)
	}

	config := checkFilesTestConfigWithScript(t, scriptsDir, scriptName, scriptFilename)
	snc := StartTestAgent(t, config)

	// capture this since t.TempDir() is dyanic
	geneartionDirectory := t.TempDir()

	t.Logf("generationDirectory: %s", geneartionDirectory)

	res := snc.RunCheck(scriptName, []string{geneartionDirectory})
	assert.Equalf(t, CheckExitOK, res.State, "state matches")
	outputString := string(res.BuildPluginOutput())
	assert.Containsf(t, outputString, "ok - Generated 3 files for testing", "output matches")
	assert.Containsf(t, outputString, "ok - Generated 0 directories for testing", "output matches")

	// check_files_perfdata_generate_whitespace_files script generates a structure that looks like this
	// The script should build a file tree that looks like this
	// Files with whitespaces are allowed on linux filesystems
	// .
	// ├── ' '
	// ├── '  '
	// └── '   '
	//
	// 0 directories, 3 files

	// Perfdata syntax for the file sizes are '<filename> size'
	// Problem is that these would add "  size", "   size" and "    size" perfdata, which may be displayed as 'size' down the line
	// These type of files hould be skipped when iterating

	res = snc.RunCheck("check_files", []string{"path=" + geneartionDirectory, "crit='size > 10Mb'", "crit='total_size > 10Mb'"})
	outputString = string(res.BuildPluginOutput())
	// This check includes the directories as well
	assert.Containsf(t, outputString, "No files found |", "output matches")

	StopTestAgent(t, snc)
}
