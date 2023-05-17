package snclient

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	AvailableChecks["check_files"] = CheckEntry{"check_files", new(CheckFiles)}
}

type CheckFiles struct{}

/* check_files
 * Description: Check the files in the directory
 */
func (l *CheckFiles) Check(_ *Agent, args []string) (*CheckResult, error) {
	check := &CheckData{
		result: &CheckResult{
			State: CheckExitOK,
		},
		detailSyntax: "%(name)",
		okSyntax:     "All %(count) files are ok",
		topSyntax:    "%(problem_count)/%(count) files (%(problem_list))",
		emptySyntax:  "No files found",
		emptyState:   CheckExitUnknown,
	}
	argList, err := check.ParseArgs(args)
	if err != nil {
		return nil, err
	}

	maxDepth := int64(-1)
	paths := []string{}

	// parse remaining args
	for _, arg := range argList {
		switch arg.key {
		case "path", "file":
			paths = append(paths, arg.value)
		case "paths":
			paths = append(paths, strings.Split(arg.value, ",")...)
		case "max-depth":
			maxDepth, _ = strconv.ParseInt(arg.value, 10, 64)
		}
	}

	for _, checkpath := range paths {
		err := filepath.WalkDir(checkpath, func(path string, dir fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if maxDepth != -1 && dir.IsDir() && strings.Count(path, string(os.PathSeparator)) > int(maxDepth) {
				return fs.SkipDir
			}
			if dir.IsDir() {
				return nil
			}

			fileInfo, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("couldn't get Stat info on file: %v", err.Error())
			}
			fileInfoSys, ok := fileInfo.Sys().(*syscall.Win32FileAttributeData)
			if !ok {
				return fmt.Errorf("type assertion for fileInfo.Sys() failed")
			}

			check.listData = append(check.listData, map[string]string{
				"access":     time.Unix(0, fileInfoSys.LastAccessTime.Nanoseconds()).In(time.UTC).Format("2006-01-02 15:04:05"),
				"access_l":   time.Unix(0, fileInfoSys.LastAccessTime.Nanoseconds()).Format("2006-01-02 15:04:05"),
				"access_u":   time.Unix(0, fileInfoSys.LastAccessTime.Nanoseconds()).In(time.UTC).Format("2006-01-02 15:04:05"),
				"age":        strconv.FormatInt((time.Now().UnixNano()-fileInfoSys.LastWriteTime.Nanoseconds())/1e9, 10),
				"creation":   time.Unix(0, fileInfoSys.CreationTime.Nanoseconds()).In(time.UTC).Format("2006-01-02 15:04:05"),
				"creation_l": time.Unix(0, fileInfoSys.CreationTime.Nanoseconds()).Format("2006-01-02 15:04:05"),
				"creation_u": time.Unix(0, fileInfoSys.CreationTime.Nanoseconds()).In(time.UTC).Format("2006-01-02 15:04:05"),
				"file":       fileInfo.Name(),
				"filename":   fileInfo.Name(),
				"line_count": "0",
				"name":       fileInfo.Name(),
				"path":       path,
				"size":       strconv.FormatInt(fileInfo.Size(), 10),
				"type":       map[bool]string{true: "directory", false: "file"}[dir.IsDir()],
				"write":      time.Unix(0, fileInfoSys.LastWriteTime.Nanoseconds()).In(time.UTC).Format("2006-01-02 15:04:05"),
				"written":    time.Unix(0, fileInfoSys.LastWriteTime.Nanoseconds()).In(time.UTC).Format("2006-01-02 15:04:05"),
				"written_l":  time.Unix(0, fileInfoSys.LastWriteTime.Nanoseconds()).Format("2006-01-02 15:04:05"),
				"written_u":  time.Unix(0, fileInfoSys.LastWriteTime.Nanoseconds()).In(time.UTC).Format("2006-01-02 15:04:05"),
			})

			return nil
		})
		if err != nil {
			log.Debug("error walking directory: %v", err)
		}
	}

	return check.Finalize()
}
