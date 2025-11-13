package snclient

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProcInterruptsParsing(t *testing.T) {
	procInterruptsFilePaths, globFileDiscoveryError := filepath.Glob("t/proc_interrupts_files/*.txt")
	require.NoError(t, globFileDiscoveryError)

	for _, procInterruptsFile := range procInterruptsFilePaths {
		t.Run(procInterruptsFile, func(t *testing.T) {
			fileContent, fileReadError := os.ReadFile(procInterruptsFile)
			require.NoError(t, fileReadError)

			_, procInterruptsParseError := ParseProcInterrupts(string(fileContent))
			require.NoError(t, procInterruptsParseError)
		})
	}
}

func TestParseProcInterrupts1(t *testing.T) {
	filepath := filepath.Join("t", "proc_interrupts_files", "thinkpad_l16_gen2.txt")

	fileContent, fileReadError := os.ReadFile(filepath)
	require.NoError(t, fileReadError)

	procInterrupts, procInterruptsParseError := ParseProcInterrupts(string(fileContent))
	require.NoError(t, procInterruptsParseError)

	// Open the file and check the lines manually

	line, err := procInterrupts.FindInterruptByID(164)
	require.NoError(t, err)
	assert.Equal(t, uint64(6710), line.interruptCountsPerCPU[3])
	assert.Equal(t, uint64(0), line.interruptCountsPerCPU[0])
	assert.Equal(t, "IR-PCI-MSIX-0000:04:00.0", line.interruptController)
	assert.Equal(t, "04:00.0", line.interrruptPciBdf)
	assert.Equal(t, "4-edge", line.interruptPinNameVector)
	assert.Equal(t, "nvme0q4", line.interruptDeviceAndDriverName)

	line, err = procInterrupts.FindInterruptByID(200)
	require.NoError(t, err)
	assert.Equal(t, "00:14.3", line.interrruptPciBdf)
	assert.Equal(t, "6-edge", line.interruptPinNameVector)
	assert.Equal(t, "iwlwifi:queue_6", line.interruptDeviceAndDriverName)

	line, err = procInterrupts.FindInterruptByName("IWI")
	require.NoError(t, err)
	assert.Equal(t, uint64(213), line.interruptCountsPerCPU[13])
	assert.Equal(t, "IRQ work interrupts", line.interruptAcronymExtended)

	line, err = procInterrupts.FindInterruptByName("ERR")
	require.NoError(t, err)
	assert.Equal(t, uint64(0), line.interruptCountSingle)
}
