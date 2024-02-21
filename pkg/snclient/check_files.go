package snclient

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pkg/humanize"
	"pkg/utils"
)

func init() {
	AvailableChecks["check_files"] = CheckEntry{"check_files", NewCheckFiles}
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

func NewCheckFiles() CheckHandler {
	return &CheckFiles{
		pathList:    CommaStringList{},
		pattern:     "*",
		maxDepth:    int64(-1),
		timeZoneStr: "Local",
	}
}

func (l *CheckFiles) Build() *CheckData {
	return &CheckData{
		name:        "check_files",
		description: "Checks files and directories.",
		implemented: ALL,
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
		okSyntax:     "%(status) - All %(count) files are ok: (%(total_size))",
		topSyntax:    "%(status) - %(problem_count)/%(count) files (%(total_size)) %(problem_list)",
		emptySyntax:  "No files found",
		emptyState:   CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "path", description: "Path of the file"},
			{name: "access", description: "Last access time"},
			{name: "age", description: "Seconds since file was last written"},
			{name: "creation", description: "Date when file was created"},
			{name: "file", description: "Name of the file"},
			{name: "filename", description: "Name of the file"},
			{name: "name", description: "Name of the file"},
			{name: "fullname", description: "Full name of the file including path"},
			{name: "size", description: "File size in bytes"},
			{name: "type", description: "Type of item (file or directory)"},
			{name: "written", description: "Date when file was last written to"},
			{name: "write", description: "Alias for written"},
			{name: "line_count", description: "Number of lines in the files (text files)"},
			{name: "total_bytes", description: "Total size over all files in bytes"},
			{name: "total_size", description: "Total size over all files as human readable bytes"},
		},
		exampleDefault: `
Alert if there are logs older than 1 hour in /tmp:

    check_files path="/tmp" pattern="*.log" "filter=age > 1h" crit="count > 0" empty-state=0 empty-syntax="no old files found" top-syntax="found %(count) too old logs"
    OK - All 138 files are ok: (29.22 MiB) |'count'=138;500;600;0 'size'=30642669B;;;0

Check for folder size:

    check_files 'path=/tmp' 'warn=total_size > 200MiB' 'crit=total_size > 300MiB'
    OK - All 145 files are ok: (34.72 MiB) |'count'=145;;;0 'size'=36406741B;209715200;314572800;0
	`,
		exampleArgs: `'path=/tmp' 'filter=age > 3d' 'warn=count > 500' 'crit=count > 600'`,
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

	totalSize := int64(0)
	for _, checkPath := range l.paths {
		checkPath = strings.TrimSpace(checkPath)

		err := filepath.WalkDir(checkPath, func(path string, dir fs.DirEntry, err error) error {
			filename := ""
			fileEntry := map[string]string{}
			if dir != nil {
				filename = dir.Name()
				fileEntry = map[string]string{
					"path":     path,
					"file":     filename,
					"filename": filename,
					"name":     filename,
				}
				if dir.IsDir() {
					fileEntry["fullname"] = path
					fileEntry["type"] = "directory"
				} else {
					fileEntry["fullname"] = filepath.Join(path, filename)
					fileEntry["type"] = "file"
				}

				if l.maxDepth != -1 && dir.IsDir() && int64(strings.Count(path, string(os.PathSeparator))) > l.maxDepth {
					return fs.SkipDir
				}

				if dir.IsDir() {
					return nil
				}
			}

			// check filter before checking errors, maybe it is skipped anyway
			if !check.MatchMapCondition(check.filter, fileEntry, true) {
				return nil
			}

			if match, _ := filepath.Match(l.pattern, filename); !match {
				return nil
			}

			// check for errors here, maybe the file would have been filtered out anyway
			if err != nil {
				return err
			}

			fileInfo, err := dir.Info()
			if err != nil {
				return fmt.Errorf("could not stat file: %s", err.Error())
			}

			fileInfoSys, err := getCheckFileTimes(fileInfo)
			if err != nil {
				return fmt.Errorf("type assertion for fileInfo.Sys() failed")
			}

			fileEntry["access"] = fileInfoSys.Atime.In(timeZone).Format("2006-01-02 15:04:05 MST")
			fileEntry["age"] = fmt.Sprintf("%d", time.Now().Unix()-fileInfoSys.Mtime.Unix())
			fileEntry["creation"] = fileInfoSys.Ctime.In(timeZone).Format("2006-01-02 15:04:05 MST")
			fileEntry["size"] = fmt.Sprintf("%d", fileInfo.Size())
			fileEntry["write"] = fileInfoSys.Mtime.In(timeZone).Format("2006-01-02 15:04:05 MST")
			fileEntry["written"] = fileInfoSys.Mtime.In(timeZone).Format("2006-01-02 15:04:05 MST")

			if hasLineCount {
				// check filter before doing even slower things
				if !check.MatchMapCondition(check.filter, fileEntry, true) {
					return nil
				}

				fileHandler, err := os.Open(path)
				if err != nil {
					return fmt.Errorf("could not open file %s: %s", path, err.Error())
				}
				fileEntry["line_count"] = fmt.Sprintf("%d", utils.LineCounter(fileHandler))
				fileHandler.Close()
			}

			if check.MatchMapCondition(check.filter, fileEntry, false) {
				check.listData = append(check.listData, fileEntry)
				totalSize += fileInfo.Size()
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("error walking directory %s: %s", checkPath, err.Error())
		}
	}

	check.details = map[string]string{
		"total_bytes": fmt.Sprintf("%d", totalSize),
		"total_size":  humanize.IBytesF(uint64(totalSize), 2),
	}

	check.result.Metrics = append(check.result.Metrics,
		&CheckMetric{
			Name:     "count",
			Value:    int64(len(check.listData)),
			Warning:  check.warnThreshold,
			Critical: check.critThreshold,
			Min:      &Zero,
		},
		&CheckMetric{
			ThresholdName: "total_size",
			Name:          "size",
			Value:         totalSize,
			Unit:          "B",
			Warning:       check.warnThreshold,
			Critical:      check.critThreshold,
			Min:           &Zero,
		},
	)

	return check.Finalize()
}
