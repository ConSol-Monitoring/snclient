package snclient

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/humanize"
)

func init() {
	AvailableChecks["check_swap_io"] = CheckEntry{"check_swap_io", NewCheckSwapIO}
}

type CheckSwapIO struct {
	lookback int64
}

const (
	defaultLookbackSwapIO = int64(60)
)

func NewCheckSwapIO() CheckHandler {
	return &CheckSwapIO{
		lookback: defaultLookbackSwapIO,
	}
}

func (l *CheckSwapIO) Build() *CheckData {
	return &CheckData{
		name:         "check_swap_io",
		description:  `Checks the swap Input / Output rate on the host.`,
		implemented:  ALL,
		hasInventory: ListInventory,
		result: &CheckResult{
			State: CheckExitOK,
		},
		args: map[string]CheckArgument{
			"lookback": {value: &l.lookback, isFilter: true, description: fmt.Sprintf("Lookback period for the value change rate calculations, given in seconds. Default: %d", defaultLookbackSwapIO)},
		},
		defaultWarning:  "",
		defaultCritical: "",
		okSyntax:        "%(status) - %(list)",
		detailSyntax:    "%(swap_count) swaps >%(swap_in_rate)/s <%(swap_out_rate)/s",
		topSyntax:       "%(status) - %(list)",
		attributes: []CheckAttribute{
			{name: "swap_count", description: "Count of swap partitions"},
			{name: "swap_in_rate_bytes", description: "Swap/Pages being brought in", unit: UByte},
			{name: "swap_in_rate", description: "Swap/Pages being sent out", unit: UByte},
			{name: "swap_out_rate_bytes", description: "Swap/Pages being brought in", unit: UByte},
			{name: "swap_out_rate", description: "Swap/Pages being sent out", unit: UByte},
		},
		exampleDefault: `
	check_swap_io
	OK - 1 swaps >22.98 KiB/s <18.58 MiB/s |'swap_in'=54024851456c;;;0; 'swap_out'=179375353856c;;;0;
`,
		exampleArgs: "",
	}
}

func (l *CheckSwapIO) Check(_ context.Context, snc *Agent, check *CheckData, _ []Argument) (*CheckResult, error) {
	switch runtime.GOOS {
	case "windows":
		return nil, fmt.Errorf("swap IO is not supported on windows")
	default:
	}

	l.addSwapRate(check, snc)

	return check.Finalize()
}

//nolint:funlen // the function is simple enough
func (l *CheckSwapIO) addSwapRate(check *CheckData, snc *Agent) {
	// snclient counters start periodicallysaving the swap in and out numbers after proram start

	entry := make(map[string]string)

	swaps := readSwaps()

	entry["swap_count"] = fmt.Sprintf("%d", len(swaps))

	counterCategory := "memory"
	counterKey := "swp_in"
	swapInCounter := snc.Counter.Get(counterCategory, counterKey)

	if swapInCounter == nil {
		entry["_error"] += fmt.Sprintf("There is no counter with category: %s and key: %s", counterCategory, counterKey)

		return
	}

	if err := swapInCounter.CheckRetention(time.Duration(l.lookback)*time.Second, 0); err != nil {
		message := fmt.Sprintf("memory swap_in counter can not hold the query period in s: %d, returned err: %s", l.lookback, err.Error())
		log.Trace(message)
		entry["_error"] = entry["_error"] + " | " + message
	}

	swapInLast := swapInCounter.GetLast()
	if swapInLast != nil {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				Name:          "swap_in",
				Unit:          "c",
				Value:         swapInLast.Value,
				ThresholdName: "swap_in",
				Warning:       nil,
				WarningStr:    nil,
				Critical:      nil,
				Min:           &Zero,
				Max:           nil,
				PerfConfig:    nil,
			},
		)
	} else {
		message := "swapInCounter does not have a last value"
		log.Debug(message)
		entry["_error"] = entry["_error"] + " | " + message
	}

	if swapInRate, err := swapInCounter.GetRate(time.Duration(l.lookback) * time.Second); err == nil {
		entry["swap_in_rate_bytes"] = fmt.Sprintf("%f", swapInRate)
		entry["swap_in_rate"] = humanize.IBytesF(uint64(swapInRate), 2)
	} else {
		message := fmt.Sprintf("Error during memory swap_in counter rate calculation: %s", err.Error())
		log.Debug(message)
		entry["_error"] = entry["_error"] + " | " + message
	}

	swapOutCounter := snc.Counter.Get("memory", "swp_out")
	counterKey = "swp_out"
	if swapOutCounter == nil {
		entry["_error"] += fmt.Sprintf("There is no counter with category: %s and key: %s", counterCategory, counterKey)

		return
	}

	swapOutLast := swapOutCounter.GetLast()
	if swapOutLast != nil {
		check.result.Metrics = append(check.result.Metrics,
			&CheckMetric{
				Name:          "swap_out",
				Unit:          "c",
				Value:         swapOutLast.Value,
				ThresholdName: "swap_out",
				Warning:       nil,
				WarningStr:    nil,
				Critical:      nil,
				Min:           &Zero,
				Max:           nil,
				PerfConfig:    nil,
			},
		)
	} else {
		message := "swapOutCounter does not have a last value"
		log.Debug(message)
		entry["_error "] = entry["_error"] + " | " + message
	}

	if err := swapOutCounter.CheckRetention(time.Duration(l.lookback)*time.Second, 0); err != nil {
		message := fmt.Sprintf("memory swap_out counter can not hold the query period in s: %d, returned err: %s", l.lookback, err.Error())
		log.Trace(message)
		entry["_error"] = entry["_error"] + " | " + message
	}

	if swapOutRate, err := swapOutCounter.GetRate(time.Duration(l.lookback) * time.Second); err == nil {
		entry["swap_out_rate_bytes"] = fmt.Sprintf("%f", swapOutRate)
		entry["swap_out_rate"] = humanize.IBytesF(uint64(swapOutRate), 2)
	} else {
		message := fmt.Sprintf("Error during memory swap_out counter rate calculation: %s", err.Error())
		log.Debug(message)
		entry["_error"] = entry["_error"] + " | " + message
	}

	check.listData = append(check.listData, entry)
}

type ProcSwapsLine struct {
	filename string
	_type    string
	_size    uint64
	used     uint64
	priority int32
}

func readSwaps() []ProcSwapsLine {
	swaps := make([]ProcSwapsLine, 0)

	switch runtime.GOOS {
	case "linux", "bsd", "darwin":
	default:
		return swaps
	}

	file, err := os.Open("/proc/swaps")
	// discern error if file does not exist
	if err != nil {
		return swaps
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		// skip header , which looks like this
		// Filename  Type  Size  Used  Priority
		if strings.HasPrefix(line, "Filename") {
			continue
		}

		fields := strings.Fields(line)

		if len(fields) == 5 {
			filename := fields[0]

			_type := fields[1]

			_size, err := convert.UInt64E(fields[2])
			if err != nil {
				continue
			}

			used, err := convert.UInt64E(fields[3])
			if err != nil {
				continue
			}

			priority, err := convert.Int32E(fields[4])
			if err != nil {
				continue
			}

			swaps = append(swaps, ProcSwapsLine{
				filename: filename,
				_type:    _type,
				_size:    _size,
				used:     used,
				priority: priority,
			})
		}
	}

	return swaps
}
