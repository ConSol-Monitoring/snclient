package snclient

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
)

func init() {
	AvailableChecks["check_logfile"] = CheckEntry{"check_logfile", NewCheckLogFile}
}

type CheckLogFile struct {
	FilePath         []string
	Paths            string
	LineDelimeter    string
	TimestampPattern string
	ColumnDelimter   string
	LabelPattern     []string
}

type LogLine struct {
	LineNumber int
	Content    string
}

func NewCheckLogFile() CheckHandler {
	return &CheckLogFile{}
}

func (c *CheckLogFile) Build() *CheckData {
	return &CheckData{
		implemented:  ALL,
		name:         "check_logfile",
		description:  "Checks logfiles or any other text format file for errors or other general patterns",
		detailSyntax: "%(line)", // cut to 200 chars
		okSyntax:     "%(status) - All %(count) / %(total) Lines OK",
		topSyntax:    "%(status) - %(problem_count)/%(count) lines (%(count)) %(problem_list)",
		emptySyntax:  "%(status) - No files found",
		emptyState:   CheckExitUnknown,
		args: map[string]CheckArgument{
			"file":         {value: &c.FilePath, description: "The file that should be checked"},
			"files":        {value: &c.Paths, description: "Comma separated list of files"},
			"line-split":   {value: &c.LineDelimeter, description: "Character string used to split a file into several lines (default \\n)"},
			"column-split": {value: &c.ColumnDelimter, description: "Tab slit default: \\t"},
			"label":        {value: &c.LabelPattern, description: "label:pattern => If the pattern is matched in a line the line will have the label set as detail"},
		},
		result: &CheckResult{
			State: CheckExitOK,
		},
		attributes: []CheckAttribute{
			{name: "count ", description: "Number of items matching the filter. Common option for all checks."},
			{name: "filename ", description: "The name of the file"},
			{name: "line", description: "Match the content of an entire line"},
			{name: "columnN", description: "Match the content of the N-th column only if enough columns exists"},
		},
		exampleDefault: `
		`,
		exampleArgs: ``,
	}
}

// Check implements CheckHandler.
func (c *CheckLogFile) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	c.FilePath = append(c.FilePath, strings.Split(c.Paths, ",")...)
	if len(c.FilePath) == 0 {
		return nil, fmt.Errorf("no file defined")
	}

	if snc.alreadyParsedLogfiles == nil {
		snc.alreadyParsedLogfiles = make(map[string]ParsedFile, 0)
	}

	patterns := make(map[string]*regexp.Regexp, len(c.LabelPattern))
	for _, labelPattern := range c.LabelPattern {
		parts := strings.SplitN(labelPattern, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("the label pattern is in the wrong format")
		}
		patterns[parts[0]] = regexp.MustCompile(parts[1])
	}

	totalLineCount := 0
	for _, fileNamme := range c.FilePath {
		if fileNamme == "" {
			continue
		}
		count := 0
		files, err := filepath.Glob(fileNamme)
		if err != nil {
			return nil, fmt.Errorf("could not get files for pattern %s, error was: %s", fileNamme, err.Error())
		}
		for _, fileName := range files {
			tmpCount, err := c.addFile(fileName, check, snc, patterns)
			if err != nil {
				return nil, fmt.Errorf("error for file %s, error was: %s", fileNamme, err.Error())
			}
			count += tmpCount
		}
		totalLineCount += count
	}
	check.details = map[string]string{
		"total": fmt.Sprintf("%d", totalLineCount),
	}

	return check.Finalize()
}

func (c *CheckLogFile) addFile(fileName string, check *CheckData, snc *Agent, labels map[string]*regexp.Regexp) (int, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return 0, fmt.Errorf("could not open file: %s error was: %s", fileName, err.Error())
	}
	defer file.Close()

	// If file was already parsed return immediately with 0 Bytes read and nil error
	for _, parsedFile := range snc.alreadyParsedLogfiles {
		if parsedFile.path != fileName {
			continue
		}
		// Was the file renewed, rotated?
		var info os.FileInfo
		info, err = file.Stat()
		inode := getInode(fileName)
		if err != nil {
			return 0, fmt.Errorf("could not get file information %s", err.Error())
		}
		if info.Size() <= int64(parsedFile.offset) {
			return 0, nil
		}
		if parsedFile.inode == int(inode) {
			parsedFile.offset = 0
		}
	}

	// Jump to last read bytes
	_, err = file.Seek(int64(snc.alreadyParsedLogfiles[fileName].offset), 0)

	if err != nil {
		return 0, fmt.Errorf("while skipping already read file an error occurred: %s", err.Error())
	}

	scanner := bufio.NewScanner(file)

	scanner.Split(c.getCustomSplitFunction())
	okReset := len(check.okThreshold) > 0
	lineStorage := make([]map[string]string, 0)
	var lineIndex int

	// filter each line
	for lineIndex = 0; scanner.Scan(); lineIndex++ {
		line := scanner.Text()
		entry := map[string]string{
			"filename": fileName,
			"line":     line,
		}
		// We have n Labels that all somehow need to be accessed
		// We have n Labels that all need to check on each line
		for label, pattern := range labels {
			entry[label] = pattern.FindString(line)
		}

		// get all Thresholds with prefix coulumn
		// if len(thres) > 0
		allThresh := append(check.warnThreshold, check.critThreshold...)
		var columnNumbers []int
		// Extract all needed threshold number
		numReg := regexp.MustCompile(`\d+`)

		for _, thresh := range allThresh {
			if strings.HasPrefix(thresh.keyword, "column") {
				match := numReg.FindString(thresh.keyword)
				if match == "" {
					continue
				}
				index, err := strconv.Atoi(match)
				if err != nil {
					// Something went wrong in parsing logic - should we just skipt this key?
					return 0, err
				}
				columnNumbers = append(columnNumbers, index)
			}
		}

		if len(columnNumbers) > 0 {
			cols := strings.Split(line, c.ColumnDelimter)
			var maxC int
			if len(columnNumbers) == 0 {
				maxC = 0
			} else {
				maxC = slices.Max(columnNumbers)
			}

			if len(cols) <= maxC {
				return 0, fmt.Errorf("not enough columns in log for separator and index")
			}

			// in range of number of coulumns
			// Fill entryp map
			for _, columnIndex := range columnNumbers {
				entry[fmt.Sprintf("column%d", columnIndex)] = cols[columnIndex]
			}

		}

		lineStorage = append(lineStorage, entry)
		// Do not check for OK with empty conditionlist, it would match all
		if okReset && check.MatchMapCondition(check.okThreshold, entry, true) {
			// Add and empty entry with the current line count to the listdata to keep track of linecount
			entry := map[string]string{
				"_count": fmt.Sprintf("%d", len(lineStorage)),
			}
			check.listData = append(check.listData, entry)
			lineStorage = make([]map[string]string, 0)
		}
	}
	check.listData = append(check.listData, lineStorage...)
	// Save File Size to check if lines were added
	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("could not get file information %s", err.Error())
	}
	pf := ParsedFile{path: fileName, offset: int(info.Size())}
	if runtime.GOOS == "linux" {
		pf.inode = int(getInode(fileName))
	}
	snc.alreadyParsedLogfiles[fileName] = pf

	return lineIndex, nil
}

func getInode(fileName string) uint64 {
	if runtime.GOOS == "linux" {
		var struttu syscall.Stat_t
		err := syscall.Stat(fileName, &struttu)
		if err != nil {
			return 0
		}
		return struttu.Ino
	}
	return 0
}

func (c *CheckLogFile) getCustomSplitFunction() bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if c.LineDelimeter == "\n" || c.LineDelimeter == "" {
			return bufio.ScanLines(data, atEOF)
		}
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexAny(data, c.LineDelimeter); i >= 0 {
			return i, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}

		return 0, nil, nil
	}
}
