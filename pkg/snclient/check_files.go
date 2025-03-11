package snclient

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/consol-monitoring/snclient/pkg/utils"
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
	paths    []string
	pathList CommaStringList
	pattern  string
	maxDepth int64
}

func NewCheckFiles() CheckHandler {
	return &CheckFiles{
		pathList: CommaStringList{},
		pattern:  "*",
		maxDepth: int64(-1),
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
			"path":      {value: &l.paths, description: "Path in which to search for files", isFilter: true},
			"file":      {value: &l.paths, description: "Alias for path", isFilter: true},
			"paths":     {value: &l.pathList, description: "A comma separated list of paths", isFilter: true},
			"pattern":   {value: &l.pattern, description: "Pattern of files to search for", isFilter: true},
			"max-depth": {value: &l.maxDepth, description: "Maximum recursion depth. Default: no limit. '0' disables recursion, '1' includes first sub folder level, etc..."},
			"timezone":  {description: "Sets the timezone for time metrics (default is local time)"},
		},
		detailSyntax: "%(name)",
		okSyntax:     "%(status) - All %(count) files are ok: (%(total_size))",
		topSyntax:    "%(status) - %(problem_count)/%(count) files (%(total_size)) %(problem_list)",
		emptySyntax:  "No files found",
		emptyState:   CheckExitUnknown,
		attributes: []CheckAttribute{
			{name: "path", description: "Path to the file"},
			{name: "filename", description: "Name of the file"},
			{name: "name", description: "Alias for filename"},
			{name: "file", description: "Alias for filename"},
			{name: "fullname", description: "Full name of the file including path"},
			{name: "type", description: "Type of item (file or dir)"},
			{name: "access", description: "Last access time", unit: UDate},
			{name: "creation", description: "Date when file was created", unit: UDate},
			{name: "size", description: "File size in bytes", unit: UByte},
			{name: "written", description: "Date when file was last written to", unit: UDate},
			{name: "write", description: "Alias for written", unit: UDate},
			{name: "age", description: "Seconds since file was last written", unit: UDuration},
			{name: "version", description: "Windows exe/dll file version (windows only)"},
			{name: "line_count", description: "Number of lines in the files (text files)"},
			{name: "total_bytes", description: "Total size over all files in bytes", unit: UByte},
			{name: "total_size", description: "Total size over all files as human readable bytes", unit: UByte},
			{name: "md5_checksum", description: "MD5 checksum Of the file"},
			{name: "md5_checksum", description: "SHA1 checksum Of the file"},
			{name: "md5_checksum", description: "SHA256 checksum Of the file"},
			{name: "md5_checksum", description: "SHA384 checksum Of the file"},
			{name: "md5_checksum", description: "SHA512 checksum Of the file"},
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

	for _, checkPath := range l.paths {
		if l.maxDepth == 0 {
			break
		}

		checkPath = l.normalizePath(checkPath)

		err := filepath.WalkDir(checkPath, func(path string, dirEntry fs.DirEntry, err error) error {
			return l.addFile(check, path, checkPath, dirEntry, err)
		})
		if err != nil {
			return nil, fmt.Errorf("error walking directory %s: %s", checkPath, err.Error())
		}
	}

	totalSize := uint64(0)
	for _, data := range check.listData {
		totalSize += convert.UInt64(data["size"])
	}

	if len(check.listData) > 0 || check.emptySyntax == "" {
		check.details = map[string]string{
			"total_bytes": fmt.Sprintf("%d", totalSize),
			"total_size":  humanize.IBytesF(convert.UInt64(totalSize), 2),
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
			})
	}
	if check.HasThreshold("size") || check.HasThreshold("total_size") {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				ThresholdName: "total_size",
				Name:          "size",
				Value:         totalSize,
				Unit:          "B",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
				Min:           &Zero,
			})
	}

	// skip file metrics unless show-all is set
	if check.showAll {
		l.addFileMetrics(check)
	}

	return check.Finalize()
}

func (l *CheckFiles) addFile(check *CheckData, path, checkPath string, dirEntry fs.DirEntry, err error) error { //nolint:gocyclo // Locality of behaviour
	needVersion := check.HasThreshold("version") || check.HasMacro("version")
	path = l.normalizePath(path)
	filename := filepath.Base(path)
	entry := map[string]string{
		"file":     filename,
		"filename": filename,
		"name":     filename,
		"path":     filepath.Dir(path),
		"fullname": path,
		"type":     "file",
	}

	pathDepth := l.getDepth(path, checkPath)
	log.Tracef("entry: %s (depth: %d)", path, pathDepth)

	if dirEntry != nil && dirEntry.IsDir() {
		// start path is never returned
		if path == checkPath {
			return nil
		}
		entry["type"] = "dir"
		if l.maxDepth != -1 && pathDepth > l.maxDepth {
			log.Tracef("skipping dir, max-depth reached: %s", path)

			return fs.SkipDir
		}

		if err != nil {
			// silently skip failed sub folder
			return fs.SkipDir
		}
	}

	if l.maxDepth != -1 && pathDepth > l.maxDepth {
		log.Tracef("skipping file, max-depth reached: %s", path)

		return nil
	}

	// check filter and pattern before doing more expensive things
	if match, _ := filepath.Match(l.pattern, entry["filename"]); !match {
		return nil
	}
	if !check.MatchMapCondition(check.filter, entry, true) {
		return nil
	}

	defer func() {
		if check.MatchMapCondition(check.filter, entry, false) {
			check.listData = append(check.listData, entry)
		}
	}()

	// check for errors here, maybe the file would have been filtered out anyway
	if err != nil {
		l.setError(entry, err)

		return nil
	}

	fileInfo, err := dirEntry.Info()
	if err != nil {
		if dirEntry != nil && dirEntry.IsDir() {
			return fs.SkipDir
		}
		l.setError(entry, err)

		return nil
	}

	fileInfoSys, err := getCheckFileTimes(fileInfo)
	if err != nil {
		return fmt.Errorf("type assertion for fileInfo.Sys() failed")
	}

	entry["access"] = fmt.Sprintf("%d", fileInfoSys.Atime.Unix())
	entry["age"] = fmt.Sprintf("%d", time.Now().Unix()-fileInfoSys.Mtime.Unix())
	entry["creation"] = fmt.Sprintf("%d", fileInfoSys.Ctime.Unix())
	entry["size"] = fmt.Sprintf("%d", fileInfo.Size())
	entry["write"] = fmt.Sprintf("%d", fileInfoSys.Mtime.Unix())
	entry["written"] = fmt.Sprintf("%d", fileInfoSys.Mtime.Unix())

	if needVersion {
		version, err := getFileVersion(path)
		if err != nil {
			log.Debugf("%s", err.Error())
		}
		entry["version"] = version
	}

	if check.HasThreshold("line_count") {
		// check filter before doing even slower things
		if !check.MatchMapCondition(check.filter, entry, true) {
			return nil
		}

		fileHandler, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("could not open file %s: %s", path, err.Error())
		}
		entry["line_count"] = fmt.Sprintf("%d", utils.LineCounter(fileHandler))
		fileHandler.Close()
	}

	if check.HasThreshold("md5_checksum") {
		// check filter before doing even slower things
		if !check.MatchMapCondition(check.filter, entry, true) {
			return nil
		}
		value, err := utils.MD5FileSum(path)
		if err != nil {
			return fmt.Errorf("could not open file %s: %s", path, err.Error())
		}
		entry["md5_checksum"] = value
	}
	if check.HasThreshold("sha1_checksum") {
		if !check.MatchMapCondition(check.filter, entry, true) {
			return nil
		}
		value, err := utils.Sha1FileSum(path)
		if err != nil {
			return fmt.Errorf("could not open file %s: %s", path, err.Error())
		}
		entry["sha1_checksum"] = value
	}
	if check.HasThreshold("sha256_checksum") {
		if !check.MatchMapCondition(check.filter, entry, true) {
			return nil
		}
		value, err := utils.Sha256FileSum(path)
		if err != nil {
			return fmt.Errorf("could not open file %s: %s", path, err.Error())
		}
		entry["sha256_checksum"] = value
	}
	if check.HasThreshold("sha384_checksum") {
		if !check.MatchMapCondition(check.filter, entry, true) {
			return nil
		}
		value, err := utils.Sha384FileSum(path)
		if err != nil {
			return fmt.Errorf("could not open file %s: %s", path, err.Error())
		}
		entry["sha384_checksum"] = value
	}
	if check.HasThreshold("sha512_checksum") {
		if !check.MatchMapCondition(check.filter, entry, true) {
			return nil
		}
		value, err := utils.Sha512FileSum(path)
		if err != nil {
			return fmt.Errorf("could not open file %s: %s", path, err.Error())
		}
		entry["sha512_checksum"] = value
	}

	return nil
}

func (l *CheckFiles) addFileMetrics(check *CheckData) {
	needSize := check.HasThreshold("size")
	needAge := check.HasThreshold("age")
	needAccess := check.HasThreshold("access")
	needWritten := check.HasThreshold("written")
	needLineCount := check.HasThreshold("line_count")

	for _, data := range check.listData {
		if needSize {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					ThresholdName: "size",
					Name:          data["filename"] + " " + "size",
					Value:         convert.UInt64(data["size"]),
					Unit:          "B",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				})
		}
		if needAge {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					ThresholdName: "age",
					Name:          data["filename"] + " " + "age",
					Value:         convert.UInt64(data["age"]),
					Unit:          "s",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				})
		}
		if needLineCount {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					ThresholdName: "line_count",
					Name:          data["filename"] + " " + "line_count",
					Value:         convert.UInt64(data["line_count"]),
					Unit:          "",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				})
		}
		if needAccess {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					ThresholdName: "access",
					Name:          data["filename"] + " " + "access",
					Value:         convert.UInt64(data["access"]),
					Unit:          "",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				})
		}
		if needWritten {
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					ThresholdName: "written",
					Name:          data["filename"] + " " + "written",
					Value:         convert.UInt64(data["written"]),
					Unit:          "",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				})
		}
	}
}

// normalizePath returns a trimmed path without spaces and trailing slashes or leading ./
func (l *CheckFiles) normalizePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "."+string(os.PathSeparator))
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, string(os.PathSeparator))

	return path
}

// getDepth returns path depth starting at 0 with the top folder
func (l *CheckFiles) getDepth(path, basePath string) int64 {
	if path == basePath {
		return 0
	}

	subPath := strings.TrimPrefix(path, basePath)

	return int64(strings.Count(subPath, string(os.PathSeparator)))
}

func (l *CheckFiles) setError(entry map[string]string, err error) {
	switch {
	case os.IsNotExist(err):
		entry["_error"] = fmt.Sprintf("%s: no such file or directory", entry["fullname"])
	case os.IsPermission(err):
		entry["_error"] = fmt.Sprintf("%s: file or directory not readable", entry["fullname"])
	default:
		entry["_error"] = fmt.Sprintf("%s: %s", entry["fullname"], err.Error())
	}
}
