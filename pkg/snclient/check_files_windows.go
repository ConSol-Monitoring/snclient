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

type CheckFiles struct {
	noCopy noCopy
	data   CheckData
}

/* check_files
 * Description: Check the files in the directory
 */
func (l *CheckFiles) Check(_ *Agent, args []string) (*CheckResult, error) {
	// default state: OK
	state := CheckExitOK
	l.data.detailSyntax = "%(name)"
	l.data.okSyntax = "All %(count) files are ok"
	l.data.topSyntax = "%(problem_count)/%(count) files (%(problem_list))"
	l.data.emptySyntax = "No files found"
	l.data.emptyState = CheckExitUnknown
	argList, err := ParseArgs(args, &l.data)
	if err != nil {
		return nil, fmt.Errorf("args error: %s", err.Error())
	}
	var output string
	maxDepth := int64(-1)
	var checkData map[string]string
	paths := []string{}

	// parse threshold args
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

	metrics := make([]*CheckMetric, 0)
	okList := make([]string, 0)
	warnList := make([]string, 0)
	critList := make([]string, 0)

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
				return fmt.Errorf("couldnt get Stat info on file: %v", err.Error())
			}
			fileInfoSys, ok := fileInfo.Sys().(*syscall.Win32FileAttributeData)
			if !ok {
				return fmt.Errorf("type assertion for fileInfo.Sys() failed")
			}

			mdata := map[string]string{
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
			}

			metric := CheckMetric{
				Warning:  l.data.warnThreshold,
				Critical: l.data.critThreshold,
			}
			if val, exists := mdata[l.data.critThreshold.name]; exists {
				metric.Name = l.data.critThreshold.name
				value, _ := strconv.ParseFloat(val, 64)
				metric.Value = value
			} else if val, exists := mdata[l.data.warnThreshold.name]; exists {
				metric.Name = l.data.warnThreshold.name
				value, _ := strconv.ParseFloat(val, 64)
				metric.Value = value
			}

			metrics = append(metrics, &metric)

			switch {
			case CompareMetrics(mdata, l.data.warnThreshold) && l.data.warnThreshold.value != "none":
				warnList = append(warnList, ParseSyntax(l.data.detailSyntax, mdata))
			case CompareMetrics(mdata, l.data.critThreshold) && l.data.critThreshold.value != "none":
				critList = append(critList, ParseSyntax(l.data.detailSyntax, mdata))
			default:
				okList = append(okList, ParseSyntax(l.data.detailSyntax, mdata))
			}

			return nil
		})
		if err != nil {
			log.Debug("error walking directory: %v", err)
		}
	}

	totalList := append(okList, append(warnList, critList...)...)

	switch {
	case len(critList) > 0:
		state = CheckExitCritical
	case len(warnList) > 0:
		state = CheckExitWarning
	case len(totalList) == 0:
		state = l.data.emptyState
	}

	checkData = map[string]string{
		"status":        strconv.FormatInt(state, 10),
		"count":         strconv.FormatInt(int64(len(totalList)), 10),
		"ok_list":       strings.Join(okList, ", "),
		"ok_count":      strconv.FormatInt(int64(len(okList)), 10),
		"warn_list":     strings.Join(warnList, ", "),
		"warn_count":    strconv.FormatInt(int64(len(warnList)), 10),
		"crit_list":     strings.Join(critList, ", "),
		"crit_count":    strconv.FormatInt(int64(len(critList)), 10),
		"list":          strings.Join(totalList, ", "),
		"problem_list":  strings.Join(append(critList, warnList...), ", "),
		"problem_count": strconv.FormatInt(int64(len(append(critList, warnList...))), 10),
	}

	if state == CheckExitOK {
		output = ParseSyntax(l.data.okSyntax, checkData)
	} else {
		output = ParseSyntax(l.data.topSyntax, checkData)
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: metrics,
	}, nil
}
