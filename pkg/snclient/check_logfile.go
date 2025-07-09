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
	Offset           string // Changed to string to detect if user provided it
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
			"offset":       {value: &c.Offset, description: "offset=<int> => Starting position for scanning the file (0 for beginning). This overrides any saved offset"},
			"line-split":   {value: &c.LineDelimeter, description: "Character string used to split a file into several lines (default \\n)"},
			"column-split": {value: &c.ColumnDelimter, description: "Tab split default: \\t"},
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

//nolint:gocyclo // A bit nested but at least well documented
func (c *CheckLogFile) addFile(fileName string, check *CheckData, snc *Agent, labels map[string]*regexp.Regexp) (int, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return 0, fmt.Errorf("could not open file: %s error was: %s", fileName, err.Error())
	}
	defer file.Close()

	// If file was already parsed return immediately with 0 Bytes read and nil error
	unCastedFile, ok := snc.alreadyParsedLogfiles.Load(fileName)
	var startOffset int64 // This will hold the final offset to use.
	var parseErr error

	if c.Offset != "" { //nolint:nestif // Trust me ....
		// User provided an offset string, attempt to parse it.
		var parsedInt int64
		parsedInt, parseErr = strconv.ParseInt(c.Offset, 10, 64)
		if parseErr != nil {
			return 0, fmt.Errorf("invalid offset value '%s' provided: %w", c.Offset, parseErr)
		}

		if parsedInt < 0 {
			return 0, fmt.Errorf("negative offset value '%d' not allowed", parsedInt)
		}
		startOffset = parsedInt
	} else {
		// No user-defined offset string (c.Offset is empty), try to load saved offset.
		if ok { // This 'ok' is from the Load operation.
			parsedFile, ok := unCastedFile.(ParsedFile)
			if !ok {
				return 0, fmt.Errorf("could not load already parsed files")
			}
			startOffset = int64(parsedFile.offset) // Use the saved offset.

			info, err := file.Stat() //nolint:govet // err is only used for the next if
			if err != nil {
				return 0, fmt.Errorf("could not read file stats for inode check %s: %s", fileName, err.Error())
			}

			currentInode := getInode(fileName)
			if currentInode != 0 && parsedFile.inode != 0 && currentInode != parsedFile.inode {
				startOffset = 0 // Inode changed, reset offset.
			} else {
				// Inode same (or not checkable). Check if offset is beyond file size or file truncated.
				if startOffset > info.Size() {
					startOffset = 0 // File truncated or saved offset invalid, reset.
				} else if startOffset == info.Size() && startOffset > 0 {
					// No new content, and not starting from 0.
					// Update inode if it was missing before and we have a current one.
					if parsedFile.inode == 0 && currentInode != 0 {
						parsedFile.inode = currentInode
						snc.alreadyParsedLogfiles.Store(fileName, parsedFile)
					}

					return 0, nil // No new lines to read.
				}
			}
		} else {
			// No user offset string and no saved offset (file not found in alreadyParsedLogfiles),
			// so start from the beginning.
			startOffset = 0
		}
	}

	// At this point, startOffset (int64) is determined. Seek to it if necessary.
	if startOffset > 0 {
		info, err := file.Stat() //nolint:govet // err is only used for the next if
		if err != nil {
			return 0, fmt.Errorf("could not stat file %s before seek: %s", fileName, err.Error())
		}
		if startOffset > info.Size() {
			// If desired offset is past EOF, seeking to EOF is correct.
			startOffset = info.Size()
		}
		// Perform the seek operation.
		_, err = file.Seek(startOffset, 0)
		if err != nil {
			return 0, fmt.Errorf("failed to seek to offset %d in %s: %w", startOffset, fileName, err)
		}
	}
	// If startOffset is 0, no explicit seek is needed as file is already at the beginning.

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
			if !strings.HasPrefix(thresh.keyword, "column") {
				continue
			}
			match := numReg.FindString(thresh.keyword)
			if match == "" {
				continue
			}
			var index int
			index, err = strconv.Atoi(match)
			if err != nil {
				return 0, fmt.Errorf("could not extract coulumn number from argument err: %s", err.Error())
			}
			columnNumbers = append(columnNumbers, index)
		}

		if len(columnNumbers) > 0 {
			cols := strings.Split(line, c.ColumnDelimter)
			var maxColoumns int
			if len(columnNumbers) == 0 {
				maxColoumns = 0
			} else {
				maxColoumns = slices.Max(columnNumbers)
			}

			if len(cols) <= maxColoumns {
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
		pf.inode = getInode(fileName)
	}
	snc.alreadyParsedLogfiles.Store(fileName, pf)

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
