package snclient

import (
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

type CheckFiles struct{}

func (l *CheckFiles) Check(_ *Agent, args []string) (*CheckResult, error) {
	paths := []string{}
	pathList := CommaStringList{}
	pattern := "*"
	maxDepth := int64(-1)
	check := &CheckData{
		name:        "check_files",
		description: "Checks files and directories.",
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]interface{}{
			"path":      &paths,
			"file":      &paths,
			"paths":     &pathList,
			"pattern":   &pattern,
			"max-depth": &maxDepth,
		},
		detailSyntax: "%(name)",
		okSyntax:     "%(status): All %(count) files are ok",
		topSyntax:    "%(status): %(problem_count)/%(count) files (%(problem_list))",
		emptySyntax:  "No files found",
		emptyState:   CheckExitUnknown,
	}
	_, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	paths = append(paths, pathList...)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no path specified")
	}

	hasLineCount := check.HasThreshold("line_count")
	timeZone, _ := time.Now().Zone()

	for _, checkPath := range paths {
		checkPath = strings.TrimSpace(checkPath)

		err := filepath.WalkDir(checkPath, func(path string, dir fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if maxDepth != -1 && dir.IsDir() && int64(strings.Count(path, string(os.PathSeparator))) > maxDepth {
				return fs.SkipDir
			}
			if dir.IsDir() {
				return nil
			}
			if match, _ := filepath.Match(pattern, dir.Name()); !match {
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
				"path":       path,
				"access":     fileInfoSys.Atime.UTC().Format("2006-01-02 15:04:05 UTC"),
				"access_l":   fileInfoSys.Atime.Format("2006-01-02 15:04:05 " + timeZone),
				"access_u":   fileInfoSys.Atime.UTC().Format("2006-01-02 15:04:05 UTC"),
				"age":        fmt.Sprintf("%d", time.Now().Unix()-fileInfoSys.Mtime.Unix()),
				"creation":   fileInfoSys.Ctime.UTC().Format("2006-01-02 15:04:05"),
				"creation_l": fileInfoSys.Ctime.Format("2006-01-02 15:04:05 " + timeZone),
				"creation_u": fileInfoSys.Ctime.UTC().Format("2006-01-02 15:04:05"),
				"file":       fileInfo.Name(),
				"filename":   fileInfo.Name(),
				"name":       fileInfo.Name(),
				"size":       fmt.Sprintf("%d", fileInfo.Size()),
				"type":       map[bool]string{true: "directory", false: "file"}[dir.IsDir()],
				"write":      fileInfoSys.Mtime.UTC().Format("2006-01-02 15:04:05"),
				"written":    fileInfoSys.Mtime.UTC().Format("2006-01-02 15:04:05"),
				"written_l":  fileInfoSys.Mtime.Format("2006-01-02 15:04:05 " + timeZone),
				"written_u":  fileInfoSys.Mtime.UTC().Format("2006-01-02 15:04:05"),
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
