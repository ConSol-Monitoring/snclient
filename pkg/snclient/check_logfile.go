package snclient

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/consol-monitoring/snclient/pkg/convert"
)

func init() {
	AvailableChecks["check_logfile"] = CheckEntry{"check_logfile", NewCheckLogFile}
}

var numReg = regexp.MustCompile(`\d+`)

type CheckLogFile struct {
	snc              *Agent
	FilePath         []string
	Paths            string
	LineDelimiter    string
	TimestampPattern string
	ColumnDelimiter  string
	LabelPattern     []string
	Offset           string // Changed to string to detect if user provided it
}

type LogLine struct {
	LineNumber int
	Content    string
}

func NewCheckLogFile() CheckHandler {
	return &CheckLogFile{
		LineDelimiter:   "\n",
		ColumnDelimiter: "\t",
	}
}

func (c *CheckLogFile) Build() *CheckData {
	return &CheckData{
		implemented:  ALL,
		name:         "check_logfile",
		description:  "Checks logfiles or any other text format file for errors or other general patterns",
		detailSyntax: "%(line | chomp | cut=200)", // cut after 200 chars
		listCombine:  "\n",
		okSyntax:     "%(status) - All %(count) / %(total) Lines OK",
		topSyntax:    "%(status) - %(problem_count)/%(count) lines (%(count)) %(problem_list)",
		emptySyntax:  "%(status) - No files found",
		emptyState:   CheckExitUnknown,
		args: map[string]CheckArgument{
			"file":         {value: &c.FilePath, description: "The file that should be checked"},
			"files":        {value: &c.Paths, description: "Comma separated list of files"},
			"offset":       {value: &c.Offset, description: "Starting position (in bytes) for scanning the file (0 for beginning). This overrides any saved offset"},
			"line-split":   {value: &c.LineDelimiter, description: "Character string used to split a file into several lines (default \\n)"},
			"column-split": {value: &c.ColumnDelimiter, description: "Tab split default: \\t"},
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
Alert if there are errors in the snclient log file:

    check_files files=/var/log/snclient/snclient.log 'warn=line like Warn' 'crit=line like Error'"
    OK - All 1787 / 1787 Lines OK
	`,
		exampleArgs: `'files=/var/log/snclient/snclient.log' 'warn=line like Warn'`,
	}
}

// Check implements CheckHandler.
func (c *CheckLogFile) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	c.snc = snc

	enabled, _, _ := snc.config.Section("/modules").GetBool("CheckLogFile")
	if !enabled {
		return nil, fmt.Errorf("module CheckLogFile is not enabled in /modules section")
	}

	c.FilePath = append(c.FilePath, strings.Split(c.Paths, ",")...)
	if len(c.FilePath) == 0 {
		return nil, fmt.Errorf("no file defined")
	}

	patterns := make(map[string]*regexp.Regexp, len(c.LabelPattern))
	for _, labelPattern := range c.LabelPattern {
		parts := strings.SplitN(labelPattern, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("the label pattern is in the wrong format")
		}
		var err error
		patterns[parts[0]], err = regexp.Compile(parts[1])
		if err != nil {
			return nil, fmt.Errorf("could not compile regex from patter: %s", err.Error())
		}
	}

	totalLineCount := 0
	for _, fileName := range c.FilePath {
		if fileName == "" {
			continue
		}
		count := 0
		files, err := filepath.Glob(fileName)
		if err != nil {
			return nil, fmt.Errorf("could not get files for pattern %s, error was: %s", fileName, err.Error())
		}
		for _, fileName := range files {
			tmpCount, err := c.addFile(fileName, check, patterns)
			if err != nil {
				return nil, fmt.Errorf("error for file %s, error was: %s", fileName, err.Error())
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

func (c *CheckLogFile) addFile(fileName string, check *CheckData, labels map[string]*regexp.Regexp) (int, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return 0, fmt.Errorf("could not open file: %s error was: %s", fileName, err.Error())
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("could not stat file %s: %s", fileName, err.Error())
	}

	currentInode := getInode(fileName)
	currentSize := info.Size()

	startOffset, err := c.getStartOffset(fileName, currentSize, currentInode)
	if err != nil {
		return 0, err
	}

	saveState := true
	defer func() {
		// save current position and inode
		if saveState {
			c.snc.alreadyParsedLogfiles.Store(fileName, ParsedFile{
				path:   fileName,
				offset: currentSize,
				inode:  currentInode,
			})
		}
	}()

	// seek to start offset
	if startOffset > 0 {
		if startOffset > currentSize {
			return 0, nil
		}
		_, err = file.Seek(startOffset, 0)
		if err != nil {
			saveState = false

			return 0, fmt.Errorf("failed to seek to offset %d in %s: %w", startOffset, fileName, err)
		}
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(c.getCustomSplitFunction())
	okReset := len(check.okThreshold) > 0
	lineStorage := make([]map[string]string, 0)

	columnNumbers := c.getRequiredColumnNumbers(check)

	// filter each line
	var lineIndex int
	for lineIndex = 0; scanner.Scan(); lineIndex++ {
		line := scanner.Text()
		entry := map[string]string{
			"filename": fileName,
			"line":     line,
		}

		// we have n labels that all need to check on each line
		for label, pattern := range labels {
			entry[label] = pattern.FindString(line)
		}

		if len(columnNumbers) > 0 {
			cols := strings.Split(line, c.ColumnDelimiter)
			for _, idx := range columnNumbers {
				if len(cols) > idx {
					entry[fmt.Sprintf("column%d", idx)] = cols[idx]
				} else {
					entry[fmt.Sprintf("column%d", idx)] = ""
				}
			}
		}

		lineStorage = append(lineStorage, entry)
		// Do not check for OK with empty condition list, it would match all
		if okReset && check.MatchMapCondition(check.okThreshold, entry, true) {
			// add and empty entry with the current line count to the list data to keep track of line count
			entry := map[string]string{
				"_count": fmt.Sprintf("%d", len(lineStorage)),
			}
			check.listData = append(check.listData, entry)
			lineStorage = make([]map[string]string, 0)
		}
	}
	check.listData = append(check.listData, lineStorage...)

	return lineIndex, nil
}

func (c *CheckLogFile) getStartOffset(fileName string, currentSize int64, currentInode uint64) (int64, error) {
	if c.Offset != "" {
		// user provided an offset string, attempt to parse it.
		startOffset, err := convert.Int64E(c.Offset)
		if err != nil {
			return 0, fmt.Errorf("invalid offset value '%s' provided: %s", c.Offset, err.Error())
		}
		if startOffset < 0 {
			return 0, fmt.Errorf("offset cannot be negative: %d", startOffset)
		}

		return startOffset, nil
	}

	// if file was already parsed return immediately with 0 Bytes read and nil error
	unCastedFile, alreadyParsed := c.snc.alreadyParsedLogfiles.Load(fileName)

	// new file, start over
	if !alreadyParsed {
		return 0, nil
	}

	// no user-defined offset string (c.Offset is empty), try to load saved offset.
	parsedFile, ok := unCastedFile.(ParsedFile)
	if !ok {
		return 0, fmt.Errorf("could not load already parsed files")
	}

	startOffset := parsedFile.offset

	// inode changed, reset offset.
	if currentInode != 0 && parsedFile.inode != 0 && currentInode != parsedFile.inode {
		return 0, nil
	}

	// check if offset is beyond file size or file truncated.
	if startOffset > currentSize {
		return 0, nil
	}

	return startOffset, nil
}

func (c *CheckLogFile) getCustomSplitFunction() bufio.SplitFunc {
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if c.LineDelimiter == "\n" || c.LineDelimiter == "" {
			return bufio.ScanLines(data, atEOF)
		}
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexAny(data, c.LineDelimiter); i >= 0 {
			return i, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}

		return 0, nil, nil
	}
}

func (c *CheckLogFile) getRequiredColumnNumbers(check *CheckData) []int {
	// extract all required threshold numbers
	columnNumbers := []int{}
	for _, macro := range check.AllRequiredMacros() {
		if !strings.HasPrefix(macro, "column") {
			continue
		}
		match := numReg.FindString(macro)
		if match == "" {
			continue
		}
		index := convert.Int(match)
		columnNumbers = append(columnNumbers, index)
	}

	slices.Sort(columnNumbers)
	columnNumbers = slices.Compact(columnNumbers)

	return columnNumbers
}
