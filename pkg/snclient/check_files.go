package snclient

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pkg/utils"
)

func init() {
	AvailableChecks["check_files"] = CheckEntry{"check_files", new(CheckFiles)}
}

type FileInfoUnified struct {
	Atime time.Time // Access time
	Mtime time.Time // Modify time
	Ctime time.Time // Create time
}

type CheckFiles struct {
	paths       []string
	pathList    CommaStringList
	pattern     string
	maxDepth    int64
	timeZoneStr string
}

func (l *CheckFiles) Build() *CheckData {
	l.paths = []string{}
	l.pathList = CommaStringList{}
	l.pattern = "*"
	l.maxDepth = int64(-1)
	l.timeZoneStr = "Local"

	return &CheckData{
		name:        "check_files",
		description: "Checks files and directories.",
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"path":      {value: &l.paths, description: "Path in which to search for files"},
			"file":      {value: &l.paths, description: "Alias for path"},
			"paths":     {value: &l.pathList, description: "A comma separated list of paths"},
			"pattern":   {value: &l.pattern, description: "Pattern of files to search for"},
			"max-depth": {value: &l.maxDepth, description: "Maximum recursion depth"},
			"timezone":  {value: &l.timeZoneStr, description: "Sets the timezone for time metrics (default is local time)"},
		},
		detailSyntax: "%(name)",
		okSyntax:     "%(status): All %(count) files are ok",
		topSyntax:    "%(status): %(problem_count)/%(count) files (%(problem_list))",
		emptySyntax:  "No files found",
		emptyState:   CheckExitUnknown,
	}
}

func (l *CheckFiles) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	l.paths = append(l.paths, l.pathList...)
	if len(l.paths) == 0 {
		return nil, fmt.Errorf("no path specified")
	}

	hasLineCount := check.HasThreshold("line_count")
	timeZone, err := time.LoadLocation(l.timeZoneStr)
	if err != nil {
		return nil, fmt.Errorf("couldn't find timezone: %s", l.timeZoneStr)
	}

	for _, checkPath := range l.paths {
		checkPath = strings.TrimSpace(checkPath)

		err := filepath.WalkDir(checkPath, func(path string, dir fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if l.maxDepth != -1 && dir.IsDir() && int64(strings.Count(path, string(os.PathSeparator))) > l.maxDepth {
				return fs.SkipDir
			}
			if dir.IsDir() {
				return nil
			}
			if match, _ := filepath.Match(l.pattern, dir.Name()); !match {
				return nil
			}

			fileInfo, err := dir.Info()
			if err != nil {
				return fmt.Errorf("could not stat file: %s", err.Error())
			}

			fileInfoSys, err := getCheckFileTimes(fileInfo)
			if err != nil {
				return fmt.Errorf("type assertion for fileInfo.Sys() failed")
			}

			fileEntry := map[string]string{
				"path":     path,
				"access":   fileInfoSys.Atime.In(timeZone).Format("2006-01-02 15:04:05 MST"),
				"age":      fmt.Sprintf("%d", time.Now().Unix()-fileInfoSys.Mtime.Unix()),
				"creation": fileInfoSys.Ctime.In(timeZone).Format("2006-01-02 15:04:05 MST"),
				"file":     fileInfo.Name(),
				"filename": fileInfo.Name(),
				"name":     fileInfo.Name(),
				"size":     fmt.Sprintf("%d", fileInfo.Size()),
				"type":     map[bool]string{true: "directory", false: "file"}[dir.IsDir()],
				"write":    fileInfoSys.Mtime.In(timeZone).Format("2006-01-02 15:04:05 MST"),
				"written":  fileInfoSys.Mtime.In(timeZone).Format("2006-01-02 15:04:05 MST"),
			}

			if hasLineCount {
				fileHandler, err := os.Open(path)
				if err != nil {
					return fmt.Errorf("could not open file %s: %s", path, err.Error())
				}
				fileEntry["line_count"] = fmt.Sprintf("%d", utils.LineCounter(fileHandler))
				fileHandler.Close()
			}

			check.listData = append(check.listData, fileEntry)

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error walking directory %s: %s", checkPath, err.Error())
		}
	}

	if check.HasThreshold("count") {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				Name:     "count",
				Value:    int64(len(check.listData)),
				Warning:  check.warnThreshold,
				Critical: check.critThreshold,
				Min:      &Zero,
			},
		)
	}

	return check.Finalize()
}
