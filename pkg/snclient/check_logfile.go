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

	"github.com/bmatcuk/doublestar/v4"
	"github.com/consol-monitoring/snclient/pkg/convert"
)

func init() {
	AvailableChecks["check_logfile"] = CheckEntry{"check_logfile", NewCheckLogFile}
}

var numReg = regexp.MustCompile(`\d+`)

type CheckLogFile struct {
	snc                *Agent
	FilePathPatterns   []string
	FilePathPatternsCS string
	LineDelimiter      string
	TimestampPattern   string
	ColumnDelimiter    string
	LabelPattern       []string
	Offset             string // Changed to string to detect if user provided it
}

type LogLine struct {
	LineNumber int
	Content    string
}

func NewCheckLogFile() CheckHandler {
	return &CheckLogFile{
		LineDelimiter:      "\n",
		ColumnDelimiter:    "\t",
		FilePathPatterns:   make([]string, 0),
		FilePathPatternsCS: "",
	}
}

func (c *CheckLogFile) Build() *CheckData {
	return &CheckData{
		implemented: ALL,
		name:        "check_logfile",
		description: `Checks logfiles or any other text format file for errors or other general patterns

    In order to use this plugin, you need to enable 'CheckLogFile' in the '[/modules]' section of the snclient_local.ini.

    Also, to avoid security issues, you need to set 'allowed pattern' in the '[/settings/check/logfile]'
    section of the snclient_local.ini to a comma separated list of allowed glob patterns.

    Example:
    [/settings/check/logfile]
    allowed pattern  = /var/log/**      # This allows all files recursively in /var/log/
    allowed pattern += /opt/logs/*.log  # This allows all files with .log extension in /opt/logs/

    See https://github.com/bmatcuk/doublestar#patterns for details on the pattern syntax.
`,
		detailSyntax: "%(line | chomp | cut=200)", // cut after 200 chars
		listCombine:  "\n",
		okSyntax:     "%(status) - %(count) line(s) found",
		topSyntax:    "%(status) - %(problem_count)/%(count) line(s) found",
		emptySyntax:  "%(status) - No files found",
		emptyState:   CheckExitUnknown,
		args: map[string]CheckArgument{
			"file":         {value: &c.FilePathPatterns, description: "The file that should be checked", isFilter: true},
			"files":        {value: &c.FilePathPatternsCS, description: "Comma separated list of files", isFilter: true},
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

	patterns, allowedPattern, err := c.processArguments()
	if err != nil {
		return nil, err
	}

	totalLineIndexedCount := 0
	checkedFilesWithMatchedEntries := make(map[string]int, 0)

	for _, fileName := range c.FilePathPatterns {
		if fileName == "" {
			continue
		}

		lineIndexedInThisFilePattern := 0
		files, err := filepath.Glob(fileName)
		if err != nil {
			return nil, fmt.Errorf("could not get files for pattern %s, error was: %s", fileName, err.Error())
		}

		for _, fileName := range files {
			if !c.matchPattern(fileName, allowedPattern) {
				log.Tracef("check_logfile rejecting file: %s as it does not any match patterns: %v ", fileName, allowedPattern)

				return nil, fmt.Errorf("file %s does not match any allowed pattern", fileName)
			}

			log.Debugf("check_logfile adding file: %s", fileName)
			entries, lineIndex, err := c.addFile(fileName, check, patterns)
			if err != nil {
				return nil, fmt.Errorf("error for file %s, error was: %s", fileName, err.Error())
			}
			log.Debugf("check_logfile file: %s | returned entries: %v | lines indexed: %d", fileName, entries, lineIndex)

			lineIndexedInThisFilePattern += lineIndex
			check.listData = append(check.listData, entries...)
			checkedFilesWithMatchedEntries[fileName] = len(entries)
		}

		totalLineIndexedCount += lineIndexedInThisFilePattern
	}

	check.details = map[string]string{
		"total":       fmt.Sprintf("%d", totalLineIndexedCount),
		"file_counts": c.buildFileCountsDetailString(checkedFilesWithMatchedEntries),
	}

	if check.HasThreshold("count") {
		check.addCountMetrics = true
		check.addCountMetricsToFront = true
	}

	switch {
	case len(checkedFilesWithMatchedEntries) == 0:
		check.emptySyntax = fmt.Sprintf("%%(status) - No files found to search lines in, search paths: '%s' ", strings.Join(c.FilePathPatterns, " , "))
	case len(check.listData) == 0:
		check.emptyState = CheckExitOK
		check.emptyStateSet = true
		check.emptySyntax = fmt.Sprintf("%%(status) - No matching lines found in files (%s)", check.details["file_counts"])
	default:
		check.okSyntax = fmt.Sprintf("%%(status) - All %%(count) line(s) found")
	}

	return check.Finalize()
}

func (c *CheckLogFile) processArguments() (patterns map[string]*regexp.Regexp, allowedPattern []string, err error) {
	enabled, _, _ := c.snc.config.Section("/modules").GetBool("CheckLogFile")
	if !enabled {
		return nil, nil, fmt.Errorf("module CheckLogFile is not enabled in /modules section")
	}

	if c.FilePathPatternsCS != "" {
		c.FilePathPatterns = append(c.FilePathPatterns, strings.Split(c.FilePathPatternsCS, ",")...)
	}
	if len(c.FilePathPatterns) == 0 {
		return nil, nil, fmt.Errorf("no file defined, specify some file path patterns")
	}

	patterns = make(map[string]*regexp.Regexp, len(c.LabelPattern))
	for _, labelPattern := range c.LabelPattern {
		parts := strings.SplitN(labelPattern, ":", 2)
		if len(parts) != 2 {
			return nil, nil, fmt.Errorf("the label pattern is in the wrong format")
		}
		var err error
		patterns[parts[0]], err = regexp.Compile(parts[1])
		if err != nil {
			return nil, nil, fmt.Errorf("could not compile regex from pattern: %s", err.Error())
		}
	}

	allowedPattern = c.getAllowedPattern()

	return patterns, allowedPattern, nil
}

func (c *CheckLogFile) buildFileCountsDetailString(checkedFilesWithMatchedEntries map[string]int) (fileCountDetails string) {
	type kv struct {
		file  string
		count int
	}
	sorted := make([]kv, 0, len(checkedFilesWithMatchedEntries))
	for file, count := range checkedFilesWithMatchedEntries {
		sorted = append(sorted, kv{file, count})
	}

	slices.SortFunc(sorted, func(a, b kv) int {
		if a.file < b.file {
			return -1
		}
		if a.file > b.file {
			return 1
		}
		if a.count < b.count {
			return -1
		}
		if a.count > b.count {
			return 1
		}

		return 0
	})

	detailParts := make([]string, 0, len(sorted))
	for _, item := range sorted {
		detailParts = append(detailParts, fmt.Sprintf("%s: %d", item.file, item.count))
	}

	fileCountDetails = strings.Join(detailParts, ", ")

	return fileCountDetails
}

func (c *CheckLogFile) addFile(fileName string, check *CheckData, labels map[string]*regexp.Regexp) (entries []map[string]string, lineIndex int, err error) {
	file, err := os.Open(fileName)
	if err != nil {
		return entries, 0, fmt.Errorf("could not open file: %s error was: %s", fileName, err.Error())
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return entries, 0, fmt.Errorf("could not stat file %s: %s", fileName, err.Error())
	}

	currentInode := getInode(fileName)
	currentSize := info.Size()

	startOffset, err := c.getStartOffset(fileName, currentSize, currentInode)
	if err != nil {
		return entries, 0, err
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
			return entries, 0, nil
		}
		_, err = file.Seek(startOffset, 0)
		if err != nil {
			saveState = false

			return entries, 0, fmt.Errorf("failed to seek to offset %d in %s: %w", startOffset, fileName, err)
		}
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(c.getCustomSplitFunction())
	okThresholdNotEmpty := len(check.okThreshold) > 0
	lineStorage := make([]map[string]string, 0)

	columnNumbers := c.getRequiredColumnNumbers(check)

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

		if !check.MatchMapCondition(check.filter, entry, false) {
			log.Tracef("file: %s , line : %s, did not match the filter set in the check, not adding to check.listData", fileName, line)

			continue
		}

		lineStorage = append(lineStorage, entry)

		// Do not check for OK condition if the OK condition list is empty, it would match everything
		if okThresholdNotEmpty && check.MatchMapCondition(check.okThreshold, entry, true) {
			// add and empty entry with the current line count to the list data to keep track of line count
			entry := map[string]string{
				"_count": fmt.Sprintf("%d", len(lineStorage)),
			}
			check.listData = append(check.listData, entry)
			lineStorage = make([]map[string]string, 0)
		}
	}

	return lineStorage, lineIndex, nil
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

// getRequiredColumnNumbers extracts all required column numbers from the check conditions
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

// getAllowedPattern returns the list of allowed patterns from the config
func (c *CheckLogFile) getAllowedPattern() []string {
	allowedPatternRaw, _ := c.snc.config.Section("/settings/check/logfile").GetString("allowed pattern")
	allowedPattern := strings.Split(allowedPatternRaw, ",")

	for i := range allowedPattern {
		allowedPattern[i] = strings.TrimSpace(allowedPattern[i])
	}

	return allowedPattern
}

// matchPattern checks if the given fileName matches any of the allowed patterns
func (c *CheckLogFile) matchPattern(fileName string, allowedPattern []string) bool {
	for _, pattern := range allowedPattern {
		matched, err := doublestar.PathMatch(pattern, fileName)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}

	return false
}
