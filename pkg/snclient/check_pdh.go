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
	CounterPath string
	HostName    string
	Type        string
	Instances   bool
	ExpandIndex bool
}

func NewCheckPDH() CheckHandler {
	return &CheckPDH{
		CounterPath: "Default",
	}
}

func (c *CheckPDH) Build() *CheckData {
	return &CheckData{
		implemented: Windows,
		name:        "check_pdh",
		description: "Checks pdh paths Handles WildCard Expansion",
		args: map[string]CheckArgument{
			"counter":      {value: &c.CounterPath, description: "The fully qualified Counter Name"},
			"HostName":     {value: &c.HostName, description: "The Name Of the Host Mashine in Network where the Counter should be searhced, defults to local mashine"},
			"expand-index": {value: &c.ExpandIndex, description: "Should \\4\\3 be translated?"},
			"instances":    {value: &c.Instances, description: "Expand WildCards And Fethch all instances"},
			"type":         {value: &c.Type, description: "this can be large or float depending what you expect, defualt is large "},
		},
		result: &CheckResult{
			State: CheckExitOK,
		},
		attributes: []CheckAttribute{
			{name: "count ", description: "Number of items matching the filter. Common option for all checks."},
			{name: "total ", description: "Total number of items. Common option for all checks."},
			{name: "value ", description: "The counter value (either float or int)"},
			{name: "value_f ", description: "The counter value (force float value)"},
			{name: "value_i ", description: "The counter value (force int value)"},
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
func (c *CheckPDH) Check(ctx context.Context, snc *Agent, check *CheckData, args []Argument) (*CheckResult, error) {
	var possiblePaths []string
	/*
		- If Expand Index -> Number to Names (is 4\30) *
		- Check If Counter is Valid *
		- Gather All Possible Path *
		- Add All Possible Path to Query and Save Coounter Handles in Map
		- Gather All Data
		Run checks


		- Request Formatted Values as Array - Single Values are arrays with one entry

	*/
	tmpPath := c.CounterPath
	r := regexp.MustCompile("\\d+")
	matches := r.FindAllString(c.CounterPath, -1)
	for _, m := range matches {
		index, err := strconv.Atoi(m)
		if err != nil {
			return nil, fmt.Errorf("Could not convert index error was %s", err.Error())
		}
		res, path := win.PdhLookupPerfNameByIndex(uint32(index))
		if res != win.ERROR_SUCCESS {
			return nil, fmt.Errorf("PDH Could not find given Index: %d response code: %d", index, res)
		}
		tmpPath = strings.Replace(tmpPath, m, path, 1)
	}

	// Expand Counter Path That Ends with WildCard *
	if c.Instances && strings.HasSuffix(tmpPath, "*") {
		res, paths := win.PdhExpandCounterPath(c.HostName, tmpPath, 0)
		if res != win.ERROR_SUCCESS {
			return nil, fmt.Errorf("Something Went wrong when Expanding the CounterPath Api Call Returned %d", res)
		}
		possiblePaths = append(possiblePaths, paths...)
	} else {
		possiblePaths = append(possiblePaths, tmpPath)
	}

	var hQuery win.PDH_HQUERY
	// Open Query  - Data Source = 0 => Local Mashine
	// ptr, _ := syscall.UTF16PtrFromString(c.HostName)
	ret := win.PdhOpenQuery(uintptr(0), uintptr(0), &hQuery)

	if ret != win.ERROR_SUCCESS {
		return nil, fmt.Errorf("Could not open Query, Something is wrong with the countername")
	}

	fmt.Printf("PDH Open Return Code: %d\n", ret)

	counters, err := addAllPathToCounter(hQuery, possiblePaths, c.HostName)
	if err != nil {
		return nil, err
	}

	// Collect Values For All Counters and save values in check.listData
	// TODO decide what Values are requested
	collectValuesForAllCounters(hQuery, counters, check)

	if check.HasThreshold("count") {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				Name:     "count",
				Value:    int64(len(check.listData)),
				Warning:  check.warnThreshold,
				Critical: check.critThreshold,
				Min:      &Zero,
			})
	}

	return check.Finalize()
}

func collectValuesForAllCounters(hQuery win.PDH_HQUERY, counters map[string]win.PDH_HCOUNTER, check *CheckData) {
	for counterPath, hCounter := range counters {
		var resArr [1]win.PDH_FMT_COUNTERVALUE_ITEM_LARGE // Need at least one nil pointer

		// TODO Default is large Values but should also support Float
		largeArr, ret := collectLargeValuesArray(hCounter, hQuery, resArr)
		if ret != win.ERROR_SUCCESS && ret != win.PDH_MORE_DATA && ret != win.PDH_NO_MORE_DATA {

			// return nil, fmt.Errorf("Could not collect formatted value %v", ret)
		}

		entry := map[string]string{}
		for _, v := range largeArr {
			entry["Name"] = strings.Replace(counterPath, "*", utf16PtrToString(v.SzName), 1)
			entry["Value"] = fmt.Sprintf("%d", v.FmtValue.LargeValue)
			if check.showAll {
				check.result.Metrics = append(check.result.Metrics,
					&CheckMetric{
						Name:     strings.Replace(counterPath, "*", utf16PtrToString(v.SzName), 1),
						Value:    v.FmtValue.LargeValue,
						Warning:  check.warnThreshold,
						Critical: check.critThreshold,
						Min:      &Zero,
					})
			}

			check.listData = append(check.listData, entry)
		}
	}
}

func addAllPathToCounter(hQuery win.PDH_HQUERY, possiblePaths []string, host string) (map[string]win.PDH_HCOUNTER, error) {
	counters := map[string]win.PDH_HCOUNTER{}
	// TODO PTR for hostName
	for _, path := range possiblePaths {
		var hCounter win.PDH_HCOUNTER
		ret := win.PdhAddCounter(hQuery, path, 0, &hCounter)
		if ret != win.ERROR_SUCCESS {
			return nil, fmt.Errorf("Could not Add One OF the Possible Paths to the Query path: %s, api response code: %d", path, ret)
		}
		counters[path] = hCounter
	}

	return counters, nil
}

func collectQueryData(pDH_HQUERY *win.PDH_HQUERY) uint32 {
	ret := win.PdhCollectQueryData(*pDH_HQUERY)
	if ret != win.PDH_MORE_DATA && ret != win.ERROR_SUCCESS {
		fmt.Printf("Might be something %d\n", ret)
	}
	// PDH requires for some data a double collection with a second wait between the calls See MSDN
	time.Sleep(time.Duration(1))
	ret = win.PdhCollectQueryData(*pDH_HQUERY)
	if ret != win.ERROR_SUCCESS {
		fmt.Printf("Something is wrong with the countername\n")
		return ret
	}
	return ret
}

/*
- Collect Data
- Collect formatted with size = 0 to get actual size
- if More Data -> Create Actual Array and fill
*/
func collectLargeValuesArray(hCounter win.PDH_HCOUNTER, hQuery win.PDH_HQUERY, resArr [1]win.PDH_FMT_COUNTERVALUE_ITEM_LARGE) ([]win.PDH_FMT_COUNTERVALUE_ITEM_LARGE, uint32) {
	var ret uint32
	var filledBuf []win.PDH_FMT_COUNTERVALUE_ITEM_LARGE
	var size uint32 = uint32(unsafe.Sizeof(win.PDH_FMT_COUNTERVALUE_ITEM_DOUBLE{}))
	bufferCount := uint32(0)
	if res := collectQueryData(&hQuery); res != win.ERROR_SUCCESS {
		// Error
		panic("unimplemented")
	}
	if ret = win.PdhGetFormattedCounterArrayLarge(hCounter, &size, &bufferCount, &resArr[0]); ret == win.PDH_MORE_DATA {
		//create array of size size =bufferCount * sizeOf(win.PDH_FMT_COUNTERVALUE_ITEM_LARGE)
		filledBuf = make([]win.PDH_FMT_COUNTERVALUE_ITEM_LARGE, bufferCount)
		ret = win.PdhGetFormattedCounterArrayLarge(hCounter, &size, &bufferCount, &filledBuf[0])
	}

	return filledBuf, size
}

func utf16PtrToString(ptr *uint16) string {
	if ptr == nil {
		return ""
	}
	end := unsafe.Pointer(ptr)
	n := 0
	for *(*uint16)(end) != 0 {
		end = unsafe.Pointer(uintptr(end) + unsafe.Sizeof(*ptr))
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(ptr, n))
}
