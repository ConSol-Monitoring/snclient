//go:build windows
// +build windows

package snclient

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/niemp100/win"
)

func init() {
	AvailableChecks["check_pdh"] = CheckEntry{"check_pdh", NewCheckPDH}
}

type CheckPDH struct {
	CounterPath          string
	HostName             string
	Type                 string
	Instances            bool
	ExpandIndex          bool
	EnglishFallBackNames bool
}

func NewCheckPDH() CheckHandler {
	return &CheckPDH{
		CounterPath: "Default",
	}
}

func (c *CheckPDH) Build() *CheckData {
	return &CheckData{
		implemented:  Windows,
		name:         "check_pdh",
		description:  "Checks pdh paths Handles WildCard Expansion",
		detailSyntax: "%(name)",
		okSyntax:     "%(status) - All %(count) Counter Values are ok",
		topSyntax:    "%(status) - %(problem_count)/%(count) Counter (%(count)) %(problem_list)",
		emptySyntax:  "No Counter Found",
		emptyState:   CheckExitUnknown,
		args: map[string]CheckArgument{
			"counter":      {value: &c.CounterPath, description: "The fully qualified Counter Name"},
			"host":         {value: &c.HostName, description: "The Name Of the Host Mashine in Network where the Counter should be searched, defults to local mashine"},
			"expand-index": {value: &c.ExpandIndex, description: "Should Indices be translated?"},
			"instances":    {value: &c.Instances, description: "Expand WildCards And Fethch all instances"},
			"type":         {value: &c.Type, description: "this can be large or float depending what you expect, default is large "},
			"english":      {value: &c.EnglishFallBackNames, description: "Using English Names Regardless of system Language requires Windows Vista or higher"},
		},
		result: &CheckResult{
			State: CheckExitOK,
		},
		attributes: []CheckAttribute{
			{name: "count ", description: "Number of items matching the filter. Common option for all checks."},
			{name: "value ", description: "The counter value (either float or int)"},
		},
		exampleDefault: `
		check_pdh "counter=foo" "warn=value > 80" "crit=value > 90"
		Everything looks good
		'foo value'=18;80;90
		`,
		exampleArgs: `counter=\\System\\System Up Time" "warn=value > 5" "crit=value > 9999`,
	}
}

// Check implements CheckHandler.
func (c *CheckPDH) Check(_ context.Context, _ *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	var possiblePaths []string
	var hQuery win.PDH_HQUERY
	// Open Query  - Data Source = 0 => Real Time Datasource
	ret := win.PdhOpenQuery(uintptr(0), uintptr(0), &hQuery)
	defer win.PdhCloseQuery(hQuery)

	if ret != win.ERROR_SUCCESS {
		return nil, fmt.Errorf("could not open query, something is wrong with the countername")
	}

	tmpPath := c.CounterPath
	if c.EnglishFallBackNames {
		var hCounter win.PDH_HCOUNTER
		ret = win.PdhAddEnglishCounter(hQuery, tmpPath, 0, &hCounter)
		if ret != win.ERROR_SUCCESS {
			return nil, fmt.Errorf("cannot use provided counter path as english fallback path, api response: %d", ret)
		}
		tpm, err := win.PdhGetCounterInfo(hCounter, false)
		if err != nil {
			return nil, fmt.Errorf("cannot use provided counter path as english fallback path, error: %s", err.Error())
		}
		tmpPath = tpm
	}

	// If HostName is set it needs to be part of the counter path
	if c.HostName != "" {
		tmpPath = `\\` + c.HostName + `\` + c.CounterPath
	}

	// Find Indices and replace with Performance Name
	r := regexp.MustCompile(`\\d+`)
	matches := r.FindAllString(c.CounterPath, -1)
	for _, match := range matches {
		index, err := strconv.Atoi(strings.ReplaceAll(match, `\`, ""))
		if err != nil {
			return nil, fmt.Errorf("could not convert index. error was %s", err.Error())
		}
		res, path := win.PdhLookupPerfNameByIndex(uint32(index)) //nolint:gosec // Index is small and needs  to be uint32 for system call
		if res != win.ERROR_SUCCESS {
			return nil, fmt.Errorf("could not find given index: %d response code: %d", index, res)
		}
		tmpPath = strings.Replace(tmpPath, match, "\\"+path, 1)
	}

	// Expand Counter Path That Ends with WildCard *
	if c.Instances && strings.HasSuffix(tmpPath, "*") {
		res, paths := win.PdhExpandCounterPath("", tmpPath, 0)
		if res != win.ERROR_SUCCESS {
			return nil, fmt.Errorf("something went wrong when expanding the counter path api call returned %d", res)
		}
		possiblePaths = append(possiblePaths, paths...)
	} else {
		possiblePaths = append(possiblePaths, tmpPath)
	}

	counters, err := c.addAllPathToCounter(hQuery, possiblePaths)
	if err != nil {
		return nil, fmt.Errorf("could not add all counter path to query, error: %s", err.Error())
	}

	// Collect Values For All Counters and save values in check.listData
	err = collectValuesForAllCounters(hQuery, counters, check)
	if err != nil {
		return nil, fmt.Errorf("could not get values for all counter path, error: %s", err.Error())
	}

	return check.Finalize()
}

func collectValuesForAllCounters(hQuery win.PDH_HQUERY, counters map[string]win.PDH_HCOUNTER, check *CheckData) error {
	for counterPath, hCounter := range counters {
		var resArr [1]win.PDH_FMT_COUNTERVALUE_ITEM_LARGE // Need at least one nil pointer

		largeArr, ret := collectLargeValuesArray(hCounter, hQuery, resArr)
		if ret != win.ERROR_SUCCESS && ret != win.PDH_MORE_DATA && ret != win.PDH_NO_MORE_DATA {
			return fmt.Errorf("could not collect formatted value %v", ret)
		}

		entry := map[string]string{}
		for _, fmtValue := range largeArr {
			entry["name"] = strings.Replace(counterPath, "*", utf16PtrToString(fmtValue.SzName), 1)
			entry["value"] = fmt.Sprintf("%d", fmtValue.FmtValue.LargeValue)
			if check.showAll {
				check.result.Metrics = append(check.result.Metrics,
					&CheckMetric{
						Name:          strings.Replace(counterPath, "*", utf16PtrToString(fmtValue.SzName), 1),
						ThresholdName: "value",
						Value:         fmtValue.FmtValue.LargeValue,
						Warning:       check.warnThreshold,
						Critical:      check.critThreshold,
						Min:           &Zero,
					})
			}
			if check.MatchMapCondition(check.filter, entry, true) {
				check.listData = append(check.listData, entry)
			}
		}
	}

	return nil
}

func (c *CheckPDH) addAllPathToCounter(hQuery win.PDH_HQUERY, possiblePaths []string) (map[string]win.PDH_HCOUNTER, error) {
	counters := map[string]win.PDH_HCOUNTER{}

	for _, path := range possiblePaths {
		var hCounter win.PDH_HCOUNTER
		ret := win.PdhAddCounter(hQuery, path, 0, &hCounter)
		if ret != win.ERROR_SUCCESS {
			return nil, fmt.Errorf("could not add one of the possible paths to the query: %s, api response code: %d", path, ret)
		}
		counters[path] = hCounter
	}

	return counters, nil
}

func collectQueryData(hQuery *win.PDH_HQUERY) uint32 {
	ret := win.PdhCollectQueryData(*hQuery)
	if ret != win.PDH_MORE_DATA && ret != win.ERROR_SUCCESS {
		return ret
	}
	// PDH requires a double collection with a second wait between the calls See MSDN
	time.Sleep(time.Duration(1))
	ret = win.PdhCollectQueryData(*hQuery)

	return ret
}

/*
- Collect Data
- Collect formatted with size = 0 to get actual size
- if More Data -> Create Actual Array and fill
*/
func collectLargeValuesArray(hCounter win.PDH_HCOUNTER, hQuery win.PDH_HQUERY, resArr [1]win.PDH_FMT_COUNTERVALUE_ITEM_LARGE) (values []win.PDH_FMT_COUNTERVALUE_ITEM_LARGE, apiResponseCode uint32) {
	var ret uint32
	var filledBuf []win.PDH_FMT_COUNTERVALUE_ITEM_LARGE
	size := uint32(0)
	bufferCount := uint32(0)
	if res := collectQueryData(&hQuery); res != win.ERROR_SUCCESS {
		return nil, res
	}
	if ret = win.PdhGetFormattedCounterArrayLarge(hCounter, &size, &bufferCount, &resArr[0]); ret == win.PDH_MORE_DATA {
		// create array of size = bufferCount * sizeOf(win.PDH_FMT_COUNTERVALUE_ITEM_LARGE)
		filledBuf = make([]win.PDH_FMT_COUNTERVALUE_ITEM_LARGE, bufferCount)
		ret = win.PdhGetFormattedCounterArrayLarge(hCounter, &size, &bufferCount, &filledBuf[0])
	}

	return filledBuf, ret
}

func utf16PtrToString(ptr *uint16) string {
	if ptr == nil {
		return ""
	}
	end := unsafe.Pointer(ptr)
	charCounter := 0
	for *(*uint16)(end) != 0 {
		end = unsafe.Pointer(uintptr(end) + unsafe.Sizeof(*ptr))
		charCounter++
	}

	return syscall.UTF16ToString(unsafe.Slice(ptr, charCounter))
}
