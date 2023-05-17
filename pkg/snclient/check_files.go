//go:build linux || darwin

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

	"pkg/utils"
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
			fileInfoSys, ok := fileInfo.Sys().(*syscall.Stat_t)
			if !ok {
				return fmt.Errorf("type assertion for fileInfo.Sys() failed")
			}

			fileHandler, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("couldnt open file %s: %v", path, err.Error())
			}

			check.listData = append(check.listData, map[string]string{
				"access":     time.Unix(fileInfoSys.Atim.Sec, fileInfoSys.Atim.Nsec).In(time.UTC).Format("2006-01-02 15:04:05"),
				"access_l":   time.Unix(fileInfoSys.Atim.Sec, fileInfoSys.Atim.Nsec).Format("2006-01-02 15:04:05"),
				"access_u":   time.Unix(fileInfoSys.Atim.Sec, fileInfoSys.Atim.Nsec).In(time.UTC).Format("2006-01-02 15:04:05"),
				"age":        strconv.FormatInt((time.Now().UnixNano()-(fileInfoSys.Atim.Sec*1e9)-fileInfoSys.Mtim.Nsec)/1e9, 10),
				"creation":   time.Unix(fileInfoSys.Ctim.Sec, fileInfoSys.Ctim.Nsec).In(time.UTC).Format("2006-01-02 15:04:05"),
				"creation_l": time.Unix(fileInfoSys.Ctim.Sec, fileInfoSys.Ctim.Nsec).Format("2006-01-02 15:04:05"),
				"creation_u": time.Unix(fileInfoSys.Ctim.Sec, fileInfoSys.Ctim.Nsec).In(time.UTC).Format("2006-01-02 15:04:05"),
				"file":       fileInfo.Name(),
				"filename":   fileInfo.Name(),
				"line_count": strconv.FormatInt(int64(utils.LineCounter(fileHandler)), 10),
				"name":       fileInfo.Name(),
				"path":       path,
				"size":       strconv.FormatInt(fileInfo.Size(), 10),
				"type":       map[bool]string{true: "directory", false: "file"}[dir.IsDir()],
				"write":      time.Unix(fileInfoSys.Mtim.Sec, fileInfoSys.Mtim.Nsec).In(time.UTC).Format("2006-01-02 15:04:05"),
				"written":    time.Unix(fileInfoSys.Mtim.Sec, fileInfoSys.Mtim.Nsec).In(time.UTC).Format("2006-01-02 15:04:05"),
				"written_l":  time.Unix(fileInfoSys.Mtim.Sec, fileInfoSys.Mtim.Nsec).Format("2006-01-02 15:04:05"),
				"written_u":  time.Unix(fileInfoSys.Mtim.Sec, fileInfoSys.Mtim.Nsec).In(time.UTC).Format("2006-01-02 15:04:05"),
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
