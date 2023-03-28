package snclient

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/mem"
)

func init() {
	AvailableChecks["check_memory"] = CheckEntry{"check_memory", new(check_memory)}
}

type check_memory struct {
	noCopy noCopy
}

/* check_memory todo
 * todo
 */
func (l *check_memory) Check(args []string) (*CheckResult, error) {

	argList := ParseArgs(args)

	for _, arg := range argList {
		switch arg.key {
		case "warn", "warning":
			warnTreshold := ParseTreshold(arg.value)
		case "crit", "critical":
			critTreshold := ParseTreshold(arg.value)
		}
	}

	v, _ := mem.VirtualMemory()
	s, _ := mem.SwapMemory()

	physicalM := []MetricData{
		MetricData{name: "used", value: strconv.FormatUint(v.Used)},
		MetricData{name: "free", value: strconv.FormatUint(v.Free)},
		MetricData{name: "used_pct", value: strconv.FormatFloat(v.UsedPercent, 'f', 0, 64)},
		MetricData{name: "free_pct", value: strconv.FormatUint(v.Free * 100 / v.Total)},
	}

	comittedM := []MetricData{
		MetricData{name: "used", value: strconv.FormatUint(s.Used)},
		MetricData{name: "free", value: strconv.FormatUint(s.Free)},
		MetricData{name: "used_pct", value: strconv.FormatFloat(s.UsedPercent, 'f', 0, 64)},
		MetricData{name: "free_pct", value: strconv.FormatUint(s.Free * 100 / s.Total)},
	}

	state := int64(0)
	output := "Dummy Check"

	if len(args) > 0 {
		res, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			res = CheckExitUnknown
			output = fmt.Sprintf("cannot parse state to int: %s", err)
		}

		state = res
	}

	if len(args) > 1 {
		output = strings.Join(args[0:], " ")
	}

	return &CheckResult{
		State:  state,
		Output: output,
	}, nil
}
