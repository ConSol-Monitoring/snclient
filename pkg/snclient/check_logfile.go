package snclient

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	LabelPattern     string
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
		implemented:  Windows,
		name:         "check_logfile",
		description:  "Checks logfiles or any other text format file for errors or other general patterns",
		detailSyntax: "%(label): %(line)",
		okSyntax:     "%(status) - All %(count) / %(total) Lines OK",
		topSyntax:    "%(status) - %(problem_count)/%(count) lines (%(count)) %(problem_list)",
		emptySyntax:  "%(status) - No files found",
		emptyState:   CheckExitUnknown,
		args: map[string]CheckArgument{
			"file":              {value: &c.FilePath, description: "The file that should be checked"},
			"files":             {value: &c.Paths, description: "Comma separated list of files"},
			"line-split":        {value: &c.LineDelimeter, description: "Character string used to split a file into several lines (default \\n)"},
			"comlumn-split":     {value: &c.ColumnDelimter, description: "Tab slit default: \\t"},
			"timestamp-pattern": {value: &c.TimestampPattern, description: "The pattern of the timestamps in the log"},
			"label-pattern":     {value: &c.LabelPattern, description: "label:pattern => If the pattern is matched in a line the line will have the label set as detail"},
		},
		result: &CheckResult{
			State: CheckExitOK,
		},
		attributes: []CheckAttribute{
			{name: "count ", description: "Number of items matching the filter. Common option for all checks."},
			{name: "filename ", description: "The name of the file"},
			{name: "line", description: "Match the content of an entire line"},
			{name: "column1", description: "Match the content of the first column"},
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

	totalLineCount := 0
	for _, fileNamme := range c.FilePath {
		if fileNamme == "" {
			continue
		}
		count := 0
		if strings.HasSuffix(fileNamme, "*") {
			matches, err := filepath.Glob(fileNamme)
			if err != nil {
				return nil, fmt.Errorf("could not get files for pattern %s, error was: %s", fileNamme, err.Error())
			}
			for _, match := range matches {
				tmpCount, _ := c.addFile(match, check, snc)
				count += tmpCount
			}
		} else {
			count, _ = c.addFile(fileNamme, check, snc)
		}
		totalLineCount += count
	}
	check.details = map[string]string{
		"total": fmt.Sprintf("%d", totalLineCount),
	}

	return check.Finalize()
}

func (c *CheckLogFile) addFile(fileName string, check *CheckData, snc *Agent) (int, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return 0, fmt.Errorf("could not open file: %s error was: %s", fileName, err.Error())
	}
	defer file.Close()

	// If file was already parsed return immediately with 0 Bytes read and nil error
	for _, parsedFile := range snc.alreadyParsedLogfiles {
		if parsedFile.path == fileName {
			// Was the file renewed, rotated?
			info, err := file.Stat()
			if err != nil {
				return 0, fmt.Errorf("could not get file information %s", err.Error())
			}
			if info.Size() <= int64(parsedFile.offset) {
				return 0, nil
			}
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
	var labelRegex *regexp.Regexp
	label := ""
	if c.LabelPattern != "" {
		parts := strings.Split(c.LabelPattern, ":")
		if len(parts) != 2 {
			return 0, fmt.Errorf("the label pattern is in the wrong format")
		}
		labelRegex = regexp.MustCompile(parts[1])
		label = parts[0]
	}
	// filter each line
	for lineIndex = 0; scanner.Scan(); lineIndex++ {
		line := scanner.Text()
		entry := map[string]string{
			"filename": fileName,
		}
		entry["line"] = line

		// Only match if label is set
		if label != "" && labelRegex.MatchString(line) {
			entry["label"] = label
		} else {
			entry["label"] = ""
		}

		if check.HasThreshold("column1") {
			entry["column1"] = strings.Split(line, c.ColumnDelimter)[0]
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
	snc.alreadyParsedLogfiles[fileName] = ParsedFile{path: fileName, line: lineIndex, offset: int(info.Size())}

	return lineIndex, nil
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
