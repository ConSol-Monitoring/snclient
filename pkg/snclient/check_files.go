package snclient

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
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
	pattern := "*"

	// parse remaining args
	for _, arg := range argList {
		switch arg.key {
		case "path", "file":
			paths = append(paths, arg.value)
		case "paths":
			paths = append(paths, strings.Split(arg.value, ",")...)
		case "max-depth":
			maxDepth, _ = strconv.ParseInt(arg.value, 10, 64)
		case "pattern":
			pattern = arg.value
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
			if match, _ := filepath.Match(pattern, dir.Name()); !match {
				return nil
			}

			fileInfo, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("couldnt get Stat info on file: %v", err.Error())
			}

			fileHandler, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("couldnt open file %s: %v", path, err.Error())
			}

			fileInfoSys, err := getCheckFileTimes(fileInfo)
			if err != nil {
				return fmt.Errorf("type assertion for fileInfo.Sys() failed")
			}

			check.listData = append(check.listData, map[string]string{
				"access":     fileInfoSys.Atime.UTC().Format("2006-01-02 15:04:05"),
				"access_l":   fileInfoSys.Atime.Format("2006-01-02 15:04:05"),
				"access_u":   fileInfoSys.Atime.UTC().Format("2006-01-02 15:04:05"),
				"age":        fmt.Sprintf("%d", time.Now().Unix()-fileInfoSys.Mtime.Unix()),
				"creation":   fileInfoSys.Ctime.UTC().Format("2006-01-02 15:04:05"),
				"creation_l": fileInfoSys.Ctime.Format("2006-01-02 15:04:05"),
				"creation_u": fileInfoSys.Ctime.UTC().Format("2006-01-02 15:04:05"),
				"file":       fileInfo.Name(),
				"filename":   fileInfo.Name(),
				"line_count": fmt.Sprintf("%d", utils.LineCounter(fileHandler)),
				"name":       fileInfo.Name(),
				"path":       path,
				"size":       fmt.Sprintf("%d", fileInfo.Size()),
				"type":       map[bool]string{true: "directory", false: "file"}[dir.IsDir()],
				"write":      fileInfoSys.Mtime.UTC().Format("2006-01-02 15:04:05"),
				"written":    fileInfoSys.Mtime.UTC().Format("2006-01-02 15:04:05"),
				"written_l":  fileInfoSys.Mtime.Format("2006-01-02 15:04:05"),
				"written_u":  fileInfoSys.Mtime.UTC().Format("2006-01-02 15:04:05"),
			})

			fileHandler.Close()

			return nil
		})
		if err != nil {
			log.Debug("error walking directory: %v", err)
		}
	}

	return check.Finalize()
}
