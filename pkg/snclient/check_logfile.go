package snclient

import (
	"context"
	"fmt"
	"os"
)

func init() {
	AvailableChecks["check_logfile"] = CheckEntry{"check_logfile", NewCheckLogFile}
}

type CheckLogFile struct {
	FilePath string
}

func NewCheckLogFile() CheckHandler {
	return &CheckLogFile{}
}

func (c *CheckLogFile) Build() *CheckData {
	return &CheckData{
		implemented:  Windows,
		name:         "check_logfile",
		description:  "Checks logfiles or any other text format file for errors or other general patterns",
		detailSyntax: "%(name)",
		okSyntax:     "%(status) - All %(count) files",
		topSyntax:    "%(status) - %(problem_count)/%(count) files (%(count)) %(problem_list)",
		emptySyntax:  "%(status) - No files found",
		emptyState:   CheckExitUnknown,
		args: map[string]CheckArgument{
			"file": {value: &c.FilePath, description: "The file that should be checked"},
		},
		result: &CheckResult{
			State: CheckExitOK,
		},
		attributes: []CheckAttribute{
			{name: "count ", description: "Number of items matching the filter. Common option for all checks."},
			{name: "value ", description: "The counter value (either float or int)"},
		},
		exampleDefault: `
		`,
		exampleArgs: ``,
	}
}

// Check implements CheckHandler.
func (c *CheckLogFile) Check(ctx context.Context, snc *Agent, check *CheckData, args []Argument) (*CheckResult, error) {
	if c.FilePath == "" {
		return nil, fmt.Errorf("no file defined")
	}
	file, err := os.OpenFile(c.FilePath, os.O_RDONLY, os.ModeCharDevice)
	defer file.Close()
	if err != nil {
		return nil, fmt.Errorf("could not open the file: %s", err.Error())
	}
	ps := os.Getpagesize()
	fmt.Printf("ps: %v\n", ps)

	buff := make([]byte, ps)

	bytesRead, err := file.Read(buff)

	fmt.Printf("bytesRead: %v\n", bytesRead)
	fmt.Printf("string(buff): %v\n", string(buff))

	return nil, nil
}
