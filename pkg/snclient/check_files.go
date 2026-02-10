package snclient

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"
	"unicode"

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
	paths                      []string
	pathList                   CommaStringList
	pattern                    string // constructor NewCheckFiles sets this as '*'
	maxDepth                   int64  // constructor NewCheckFiles sets this as -1
	calculateSubdirectorySizes bool   // constructor NewCheckFiles sets this as false
}

func NewCheckFiles() CheckHandler {
	return &CheckFiles{
		pathList:                   CommaStringList{},
		pattern:                    "*",
		maxDepth:                   int64(-1),
		calculateSubdirectorySizes: false,
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
			"path":    {value: &l.paths, description: "Path in which to search for files", isFilter: true},
			"file":    {value: &l.paths, description: "Alias for path", isFilter: true},
			"paths":   {value: &l.pathList, description: "A comma separated list of paths", isFilter: true},
			"pattern": {value: &l.pattern, description: "Pattern of files to search for", isFilter: true},
			"max-depth": {value: &l.maxDepth, description: "Maximum recursion depth. Default: no limit. '0' and '1' disable recursion and only include files/directories directly under path." +
				", '2' starts to include files/directories of subdirectories with given depth. "},
			"timezone": {description: "Sets the timezone for time metrics (default is local time)"},
			"calculate-subdirectory-sizes": {value: &l.calculateSubdirectorySizes, description: "For subdirectories that are found under the search paths, " +
				"calculate the subdirectory sizes based on found files. This calculation may be expensive. Default: false"},
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
			{name: "check_path", description: "Check path argument whose search led to finding this file/directory."},
			{name: "access", description: "Unix timestamp of last access time", unit: UDate},
			{name: "creation", description: "Unix timestamp when file was created", unit: UDate},
			{name: "size", description: "File size in bytes", unit: UByte},
			{name: "written", description: "Unix timestamp when file was last written to", unit: UDate},
			{name: "write", description: "Alias for written", unit: UDate},
			{name: "age", description: "Seconds since file was last written", unit: UDuration},
			{name: "version", description: "Windows exe/dll file version (windows only)"},
			{name: "line_count", description: "Number of lines in the files (text files)"},
			{name: "total_bytes", description: "Total size over all files in bytes", unit: UByte},
			{name: "total_size", description: "Total size over all files as human readable bytes", unit: UByte},
			{name: "md5_checksum", description: "MD5 checksum of the file"},
			{name: "sha1_checksum", description: "SHA1 checksum of the file"},
			{name: "sha256_checksum", description: "SHA256 checksum of the file"},
			{name: "sha384_checksum", description: "SHA384 checksum of the file"},
			{name: "sha512_checksum", description: "SHA512 checksum of the file"},
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
		checkPath = l.normalizePath(checkPath)
		log.Tracef("normalized checkPath: %s", checkPath)

		err := filepath.WalkDir(checkPath, func(path string, dirEntry fs.DirEntry, err error) error {
			return l.addFile(check, path, checkPath, dirEntry, err)
		})
		if err != nil {
			return nil, fmt.Errorf("error walking directory %s: %s", checkPath, err.Error())
		}
	}

	// Cleanup the listData if a filter is used
	if l.pattern != "*" {
		l.removeDirectoriesWithoutFilesUnder(check)
	}

	if l.calculateSubdirectorySizes {
		l.addSubdirectorySizes(check)
	}

	// file metrics are always added
	l.addFileMetrics(check)

	// general metrics are always added
	l.addGeneralMetrics(check)

	if len(l.paths) > 2 {
		l.addSearchPathMetrics(check)
	}

	if l.calculateSubdirectorySizes && l.pattern != "*" {
		l.addSubDirMetrics(check)
	}

	return check.Finalize()
}

func (l *CheckFiles) addFile(check *CheckData, path, checkPath string, dirEntry fs.DirEntry, err error) error {
	// if the search path is a directory e.g '/usr/bin' , the program assumes you are looking for files/subdirectories under it
	// therefore it does not add the search path directory to the entry list
	// if it is a file like /usr/bin/bash however, it will add that
	if dirEntry != nil && dirEntry.IsDir() && path == checkPath {
		return nil
	}

	path = l.normalizePath(path)
	filename := filepath.Base(path)
	entry := map[string]string{
		"file":       filename,
		"filename":   filename,
		"name":       filename,
		"path":       filepath.Dir(path),
		"fullname":   path,
		"type":       "file",
		"check_path": checkPath,
	}

	matchAndAdd := func() {
		if check.MatchMapCondition(check.filter, entry, false) {
			log.Tracef("path : %s, matched the map conditions appending to the check.listData", path)
			check.listData = append(check.listData, entry)
		} else {
			log.Tracef("path : %s, did not match the map conditions", path)
		}
	}

	pathDepth := l.getDepth(path, checkPath)

	if dirEntry != nil && dirEntry.IsDir() {
		entry["type"] = "dir"

		if err != nil {
			// silently skip failed subdirectory.
			// If you continue on and the error is checked later, it will add error to the entry
			// This will make tests fail.
			log.Tracef("dir: %s, an error occurred during walk: %s, skipping this directory", entry["fullname"], err.Error())

			return fs.SkipDir
		}

		// if recursion is disabled with maxDepth
		if l.maxDepth != -1 && pathDepth >= 2 {
			switch {
			case pathDepth < l.maxDepth:
				log.Tracef("dir: %s, pathDepth: %d, maxDepth: %d, possible to add dir, deferring to add", path, pathDepth, l.maxDepth)
				defer matchAndAdd()

				return nil
			case pathDepth == l.maxDepth:
				log.Tracef("dir: %s, pathDepth: %d, maxDepth: %d, possible to add dir, deferring to add and returning fs.Skipdir", path, pathDepth, l.maxDepth)
				defer matchAndAdd()

				return fs.SkipDir
			default:
				log.Tracef("dir: %s, pathDepth: %d, maxDepth: %d, can not add dir, returning fs.SkipDir", path, pathDepth, l.maxDepth)

				return fs.SkipDir
			}
		}
	}

	// if recursion is disabled with maxDepth
	if l.maxDepth != -1 && pathDepth >= 2 && pathDepth > l.maxDepth {
		log.Tracef("skipping file: %s, pathDepth: %d, max-depth:%d is lower", path, pathDepth, l.maxDepth)

		return nil
	}

	// check filter and pattern before doing more expensive things
	// pattern matching is only done on files, directories are always added
	// directories that do not have any matched files under them are later removed
	if match, _ := filepath.Match(l.pattern, entry["filename"]); entry["type"] == "file" && !match {
		log.Tracef("filename: %s did not match the pattern: %s , skipping", entry["filename"], l.pattern)

		return nil
	}
	if !check.MatchMapCondition(check.filter, entry, true) {
		return nil
	}

	defer matchAndAdd()

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

	needVersion := check.HasThreshold("version") || check.HasMacro("version")
	if needVersion {
		version, err := getFileVersion(path)
		if err != nil {
			log.Debugf("%s", err.Error())
		}
		entry["version"] = version
	}

	if err := checkSlowFileOperations(check, entry, path); err != nil {
		return err
	}

	return nil
}

func checkSlowFileOperations(check *CheckData, entry map[string]string, path string) error {
	// check filter before doing even slower things
	if !check.MatchMapCondition(check.filter, entry, true) {
		return nil
	}
	if check.HasThreshold("line_count") {
		fileHandler, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("could not open file %s: %s", path, err.Error())
		}
		entry["line_count"] = fmt.Sprintf("%d", utils.LineCounter(fileHandler))
		fileHandler.Close()
	}

	if check.HasThreshold("md5_checksum") {
		value, err := utils.MD5FileSum(path)
		if err != nil {
			return fmt.Errorf("could not open file %s: %s", path, err.Error())
		}
		entry["md5_checksum"] = value
	}
	if check.HasThreshold("sha1_checksum") {
		value, err := utils.Sha1FileSum(path)
		if err != nil {
			return fmt.Errorf("could not open file %s: %s", path, err.Error())
		}
		entry["sha1_checksum"] = value
	}
	if check.HasThreshold("sha256_checksum") {
		value, err := utils.Sha256FileSum(path)
		if err != nil {
			return fmt.Errorf("could not open file %s: %s", path, err.Error())
		}
		entry["sha256_checksum"] = value
	}
	if check.HasThreshold("sha384_checksum") {
		value, err := utils.Sha384FileSum(path)
		if err != nil {
			return fmt.Errorf("could not open file %s: %s", path, err.Error())
		}
		entry["sha384_checksum"] = value
	}
	if check.HasThreshold("sha512_checksum") {
		value, err := utils.Sha512FileSum(path)
		if err != nil {
			return fmt.Errorf("could not open file %s: %s", path, err.Error())
		}
		entry["sha512_checksum"] = value
	}

	return nil
}

// The WalkDir normally adds every directory and files under the search path.
// If a pattern is specified, this prevents files that dont match the pattern to be skipped.
// This can lead to some directories being in the listData, while not having any matched files under them.
// This function cleans those directories up.
func (l *CheckFiles) removeDirectoriesWithoutFilesUnder(check *CheckData) {
	fileFilepaths := make([]string, 0)

	for _, data := range check.listData {
		if data["type"] == "file" {
			fileFilepaths = append(fileFilepaths, data["fullname"])
		}
	}

	newListData := make([]map[string]string, 0)

	for _, data := range check.listData {
		// only filter the directories, files are automatically added
		if data["type"] == "dir" {
			hasFilesUnder := false
			for _, fileFilepath := range fileFilepaths {
				prefixToMatch := fmt.Sprintf("%s%c", data["fullname"], os.PathSeparator)
				rest, found := strings.CutPrefix(fileFilepath, prefixToMatch)
				if found && rest != "" {
					hasFilesUnder = true

					break
				}
			}
			if hasFilesUnder {
				newListData = append(newListData, data)
			} else {
				log.Debugf("Skipping directory from the new listData, as it does not have any files found under it: %s", data["fullname"])
			}
		} else {
			newListData = append(newListData, data)
		}
	}

	check.listData = newListData
}

// Files are checked by their individual attributes, with directories we have to count and size them up
// This function should be called with the final check.listData
func (l *CheckFiles) addGeneralMetrics(check *CheckData) {
	// totalSize is always calculated, even if there are one/multiple search paths, and they point to files/directories
	totalSize := uint64(0)
	for _, data := range check.listData {
		// directory entries could have their "size" set.
		// this can lead to including a file multiple times in the count. only add files to totalSize
		if data["type"] == "file" {
			totalSize += convert.UInt64(data["size"])
		}
	}

	// this is added to check.details and not a metric
	if len(check.listData) > 0 || check.emptySyntax == "" {
		check.details = map[string]string{
			"total_bytes": fmt.Sprintf("%d", totalSize),
			"total_size":  humanize.IBytesF(convert.UInt64(totalSize), 2),
		}
	}

	// files do not have a 'count' atrribute, so this wont collide like 'size' would. No need for 'totalCount'
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

	// entries in listData have a 'size' attribute. This is filled for files directly, with folders they have to be calculated after the walk has ended.
	// total_size argument is the recommended way for thresholds, if they want to work with size summation of matched entries
	if check.HasThreshold("total_size") {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				ThresholdName: "total_size",
				Name:          "total_size",
				Value:         totalSize,
				Unit:          "B",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
				Min:           &Zero,
			})
	}

	if check.HasThreshold("size") {
		log.Warn("check_files - Using 'size' in a threshold argument meant to mean \"summation of all found files sizes\" is wrong. " +
			"This collides with each file entry 'size' attribute during checks. Using 'size' in a condition will check each files 'size' attribute. " +
			"If you want to check for the sum of sizes, use 'total_size' in your condition instead. ")
	}
}

// this check might be called with multiple paths. calculate their sizes individually and add as a metric
func (l *CheckFiles) addSearchPathMetrics(check *CheckData) {
	// this calculations are not accurate, as we are not including the directories sizes themselves
	for _, checkPath := range l.paths {
		checkPathNormalized := l.normalizePath(checkPath)
		pathSize := uint64(0)
		for _, data := range check.listData {
			// 1. if we look at the paths, a file might be found under multiple search paths
			// instead we save and check the path that led to this file being found
			// 2. directory entries could have their "size" set.
			// this can lead to including a file multiple times in the count. only add files to totalSize
			if data["type"] == "file" && checkPathNormalized == data["check_path"] {
				pathSize += convert.UInt64(data["size"])
			}
		}

		pathMetricName := "size " + checkPath
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				ThresholdName: pathMetricName,
				Name:          pathMetricName,
				Value:         pathSize,
				Unit:          "B",
				Warning:       check.warnThreshold,
				Critical:      check.critThreshold,
				Min:           &Zero,
			})
	}
}

// if specified, calculate the sizes of the subdirectories, that are not exactly search paths
// this function modifies the entries in the listData. It does not add metrics
// the sizes it calculcates are not accurate. it is just a sum of files under them.
// there are logical/physical sizes, disk block size, indexing, compression etc. to consider.
func (l *CheckFiles) addSubdirectorySizes(check *CheckData) {
	for _, subDirData := range check.listData {
		if subDirData["type"] != "dir" {
			continue
		}
		if slices.Contains(l.paths, subDirData["fullname"]) {
			continue
		}
		subDirSize := uint64(0)
		for _, data := range check.listData {
			if data["type"] != "file" {
				continue
			}
			prefixToMatch := fmt.Sprintf("%s%c", subDirData["fullname"], os.PathSeparator)
			rest, found := strings.CutPrefix(data["fullname"], prefixToMatch)
			if found && rest != "" {
				subDirSize += convert.UInt64(data["size"])
			}
		}

		subDirData["size"] = fmt.Sprintf("%d", subDirSize)
	}
}

// if specified, calculate the sizes of the directories, that are not exactly search paths
// the entries should have a valid "size" attributes. populate them beforehand.
func (l *CheckFiles) addSubDirMetrics(check *CheckData) {
	for _, data := range check.listData {
		if data["type"] == "dir" {
			subDirMetricName := data["fullname"] + " size"
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					ThresholdName: subDirMetricName,
					Name:          subDirMetricName,
					Value:         data["size"],
					Unit:          "B",
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				})
		}
	}
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

// normalizePath returns a trimmed path without spaces, trailing / or \, leading ./ or .\
func (l *CheckFiles) normalizePath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "."+string(os.PathSeparator))
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimSuffix(path, string(os.PathSeparator))

	// Special handling for Windows drive letters,
	// Files directly under the drive do not have seperators, e.g: C:pagefile.sys
	// This confuses the depth calculation
	if runtime.GOOS == "windows" {
		// C: -> C:\
		if len(path) == 2 && unicode.IsUpper(rune(path[0])) && path[1] == ':' {
			// Ensure drive letters always have a backslash: "C:" -> "C:\"
			return path + string(os.PathSeparator)
		}
		// "C:example" -> "C:\example".
		if len(path) >= 3 && unicode.IsUpper(rune(path[0])) && path[1] == ':' && path[2] != '\\' {
			winBasePath := path[:2] + string('\\') + path[2:]

			return winBasePath
		}
	}

	return path
}

// getDepth returns path depth starting at 0 with for the basePath
func (l *CheckFiles) getDepth(path, basePath string) int64 {
	// both the path and BasePath are normalized once according to CheckFiles.normalizePath()

	// Windows example:
	// basePath: C:\foo
	// path: C:\foo -> 0
	// path: C:\foo\bar -> 1
	// path: C:\foo\bar\baz -> 2

	if path == basePath {
		return 0
	}

	if !strings.HasPrefix(path, basePath) {
		log.Tracef("basePath: %s, is not a prefix of path: %s", basePath, path)

		return -1
	}

	subPath := strings.TrimPrefix(path, basePath)
	seperatorCount := int64(strings.Count(subPath, string(os.PathSeparator)))
	depthCorrection := 0

	// On Windows specifically, the paths have a problem when basePath is a drive letter.
	// basePath: C:\
	// path: C:\pagefile.sys -> 0 seperators , even though it is under the drive.
	// path: C:\Windows\notepad.exe -> 1 seperators, even though it is under the Windows folder.
	// If you consider the drive itself as a folder, stuff under it should have depth
	if runtime.GOOS == "windows" && len(basePath) == 3 && unicode.IsUpper(rune(path[0])) && path[1] == ':' && path[2] == '\\' {
		depthCorrection++
	}
	// Implement something similar in Unixes?

	depth := seperatorCount + int64(depthCorrection)

	log.Tracef("path: %s, basePath: %s, subPath: %s, seperatorCount: %d, depthCorrection: %d, depth: %d", path, basePath, subPath, seperatorCount, depthCorrection, depth)

	return depth
}

func (l *CheckFiles) setError(entry map[string]string, err error) {
	switch {
	case os.IsNotExist(err):
		entry["_error"] = fmt.Sprintf("%s: no such file or directory", entry["fullname"])
	case os.IsPermission(err):
		entry["_error"] = fmt.Sprintf("%s: file or directory not readable", entry["fullname"])
	default:
		// Handle *fs.PathError specifically
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			switch {
			case errors.Is(pathErr, syscall.ENOENT):
				entry["_error"] = fmt.Sprintf("%s: no such file or directory", entry["fullname"])
			case errors.Is(pathErr, syscall.EACCES):
				entry["_error"] = fmt.Sprintf("%s: file or directory not readable", entry["fullname"])
			case errors.Is(pathErr, syscall.EPERM):
				entry["_error"] = fmt.Sprintf("%s: file or directory not readable", entry["fullname"])
			default:
				entry["_error"] = fmt.Sprintf("%s: %s", entry["fullname"], pathErr.Err.Error())
			}
		} else {
			entry["_error"] = fmt.Sprintf("%s: %s", entry["fullname"], err.Error())
		}
	}
}
