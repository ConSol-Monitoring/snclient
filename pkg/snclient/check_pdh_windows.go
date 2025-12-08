package snclient

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unsafe"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/pdh"
	"golang.org/x/sys/windows"
)

const querySleepDuration = 500 * time.Millisecond

type PdhValue struct {
	CounterInstance string
	Value           int64
}

// Check implements CheckHandler.
func (c *CheckPDH) check(_ context.Context, _ *Agent, check *CheckData, args []Argument) (*CheckResult, error) {
	// If the counter path is empty we need to parse the argument ourself for the optional alias case counter:alias=...
	if c.CounterPath == "" {
		err := c.parseCheckSpecificArgs(args)
		if err != nil {
			return nil, err
		}
	}
	var possiblePaths []string
	var hQuery pdh.PDH_HQUERY
	// Open Query  - Data Source = 0 => Real Time Datasource
	ret := pdh.PdhOpenQuery(0, 0, &hQuery)
	if ret != pdh.ERROR_SUCCESS {
		return nil, fmt.Errorf("could not open query, something is wrong with the countername")
	}
	defer pdh.PdhCloseQuery(hQuery)

	tmpPath := c.CounterPath
	if c.EnglishFallBackNames {
		var hCounter pdh.PDH_HCOUNTER
		ret = pdh.PdhAddEnglishCounter(hQuery, tmpPath, 0, &hCounter)
		if ret != pdh.ERROR_SUCCESS {
			return nil, fmt.Errorf("cannot use provided counter path as english fallback path, api response: %d", ret)
		}
		tpm, err := pdh.PdhGetCounterInfo(hCounter, false)
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
	r := regexp.MustCompile(`\d+`)
	matches := r.FindAllString(c.CounterPath, -1)
	for _, match := range matches {
		index, err := convert.UInt32E(strings.ReplaceAll(match, `\`, ""))
		if err != nil {
			return nil, fmt.Errorf("could not convert index. error was %s", err.Error())
		}
		res, path := pdh.PdhLookupPerfNameByIndex(index)
		if res != pdh.ERROR_SUCCESS {
			return nil, fmt.Errorf("could not find given index: %d response code: %d", index, res)
		}
		tmpPath = strings.Replace(tmpPath, match, path, 1)
	}

	// Expand Counter Path That Ends with WildCard *
	if c.Instances && strings.HasSuffix(tmpPath, "*") {
		res, paths := pdh.PdhExpandCounterPath("", tmpPath, 0)
		if res != pdh.ERROR_SUCCESS {
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
	err = c.collectValuesForAllCounters(hQuery, counters, check)
	if err != nil {
		return nil, fmt.Errorf("could not get values for all counter path, error: %s", err.Error())
	}

	return check.Finalize()
}

func (c *CheckPDH) parseCheckSpecificArgs(args []Argument) error {
	carg := args[0]
	parts := strings.Split(carg.key, ":")
	if len(parts) < 2 {
		return fmt.Errorf("no counter defined")
	}
	counterKey := parts[0]
	alias := parts[1]

	if !strings.EqualFold(counterKey, "counter") {
		return fmt.Errorf("expected a counter definition")
	}
	c.OptionalAlias = alias
	c.CounterPath = carg.value

	return nil
}

func (c *CheckPDH) collectValuesForAllCounters(hQuery pdh.PDH_HQUERY, counters map[string]pdh.PDH_HCOUNTER, check *CheckData) error {
	for counterPath, hCounter := range counters {
		largeArr, ret := collectLargeValuesArray(hCounter, hQuery)
		if ret != pdh.ERROR_SUCCESS && ret != pdh.PDH_MORE_DATA && ret != pdh.PDH_NO_MORE_DATA {
			return fmt.Errorf("could not collect formatted value %v", ret)
		}
		entry := map[string]string{}
		for _, fmtValue := range largeArr {
			var name string
			if c.OptionalAlias != "" {
				name = c.OptionalAlias
			} else {
				name = strings.Replace(counterPath, "*", fmtValue.CounterInstance, 1)
			}
			entry["name"] = name
			entry["value"] = fmt.Sprintf("%d", fmtValue.Value)
			check.result.Metrics = append(check.result.Metrics,
				&CheckMetric{
					Name:          name + "_value",
					ThresholdName: "value",
					Value:         fmtValue.Value,
					Warning:       check.warnThreshold,
					Critical:      check.critThreshold,
					Min:           &Zero,
				})
			if check.MatchMapCondition(check.filter, entry, true) {
				check.listData = append(check.listData, entry)
			}
		}
	}

	return nil
}

func (c *CheckPDH) addAllPathToCounter(hQuery pdh.PDH_HQUERY, possiblePaths []string) (map[string]pdh.PDH_HCOUNTER, error) {
	counters := map[string]pdh.PDH_HCOUNTER{}

	for _, path := range possiblePaths {
		var hCounter pdh.PDH_HCOUNTER
		ret := pdh.PdhAddCounter(hQuery, path, 0, &hCounter)
		if ret != pdh.ERROR_SUCCESS {
			return nil, fmt.Errorf("could not add one of the possible paths to the query: %s, api response code: %d", path, ret)
		}
		counters[path] = hCounter
	}

	return counters, nil
}

func collectQueryData(hQuery *pdh.PDH_HQUERY) uint32 {
	ret := pdh.PdhCollectQueryData(*hQuery)
	if ret != pdh.PDH_MORE_DATA && ret != pdh.ERROR_SUCCESS {
		return ret
	}
	// PDH requires a double collection with a second wait between the calls See MSDN
	time.Sleep(querySleepDuration)
	ret = pdh.PdhCollectQueryData(*hQuery)

	return ret
}

/*
- Collect Data
- Collect formatted with size = 0 to get actual size
- if More Data -> Create Actual Array and fill
*/
func collectLargeValuesArray(hCounter pdh.PDH_HCOUNTER, hQuery pdh.PDH_HQUERY) (values []PdhValue, apiResponseCode uint32) {
	var bufCount uint32
	var bufSize uint32
	size := uint32(unsafe.Sizeof(pdh.PDH_FMT_COUNTERVALUE_ITEM_LARGE{}))
	var emptyBuf [1]pdh.PDH_FMT_COUNTERVALUE_ITEM_LARGE // need at least 1 addressable null ptr.
	if res := collectQueryData(&hQuery); res != pdh.ERROR_SUCCESS {
		return nil, res
	}
	returnArray := []PdhValue{}
	if ret := pdh.PdhGetFormattedCounterArrayLarge(hCounter, &bufSize, &bufCount, &emptyBuf[0]); ret == pdh.PDH_MORE_DATA {
		filledBuf := make([]pdh.PDH_FMT_COUNTERVALUE_ITEM_LARGE, bufCount*size)
		pdh.PdhGetFormattedCounterArrayLarge(hCounter, &bufSize, &bufCount, &filledBuf[0])
		for i := range int(bufCount) {
			returnArray = append(returnArray, PdhValue{
				CounterInstance: windows.UTF16PtrToString(filledBuf[i].SzName),
				Value:           filledBuf[i].FmtValue.LargeValue,
			})
		}
	}

	return returnArray, pdh.ERROR_SUCCESS
}
