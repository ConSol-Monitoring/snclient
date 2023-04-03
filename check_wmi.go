package snclient

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

func init() {
	AvailableChecks["check_wmi"] = CheckEntry{"check_wmi", new(CheckWMI)}
}

type CheckWMI struct {
	noCopy noCopy
}

type WMI struct {
	key   string
	value string
}

func QueryWMI(query string) ([]WMI, string) {

	var ret []WMI
	var output []string

	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	unknown, _ := oleutil.CreateObject("WbemScripting.SWbemLocator")
	defer unknown.Release()

	wmi, _ := unknown.QueryInterface(ole.IID_IDispatch)
	defer wmi.Release()

	// service is a SWbemServices
	serviceRaw, _ := oleutil.CallMethod(wmi, "ConnectServer")
	service := serviceRaw.ToIDispatch()
	defer service.Release()

	// result is a SWBemObjectSet
	resultRaw, _ := oleutil.CallMethod(service, "ExecQuery", query)
	result := resultRaw.ToIDispatch()
	defer result.Release()

	countVar, _ := oleutil.GetProperty(result, "Count")
	count := int(countVar.Val)

	re := regexp.MustCompile(`\w+\s+((?:\w+\s*,\s*)*\w+)`)
	values := strings.Split(re.FindStringSubmatch(query)[1], ",")

	for i := 0; i < count; i++ {
		// item is a SWbemObject, but really a Win32_Process
		itemRaw, _ := oleutil.CallMethod(result, "ItemIndex", i)
		item := itemRaw.ToIDispatch()
		defer item.Release()

		for _, v := range values {
			s, _ := oleutil.GetProperty(item, strings.TrimSpace(v))
			ret = append(ret, WMI{key: v, value: s.ToString()})
			output = append(output, s.ToString())
		}

	}

	return ret, strings.Join(output, ", ")

}

/* check_service todo
 * todo
 */
func (l *CheckWMI) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	argList := ParseArgs(args)
	var warnTreshold Treshold
	var critTreshold Treshold
	var query string

	// parse treshold args
	for _, arg := range argList {
		switch arg.key {
		case "warn", "warning":
			warnTreshold = ParseTreshold(arg.value)
		case "crit", "critical":
			critTreshold = ParseTreshold(arg.value)
		case "query":
			query = arg.value
		}
	}

	// query wmi
	querydata, output := QueryWMI(query)

	mdata := []MetricData{}
	perfMetrics := []*CheckMetric{}

	for _, d := range querydata {
		mdata = append(mdata, MetricData{name: d.key, value: d.value})
		if d.key == warnTreshold.name || d.key == critTreshold.name {
			value, _ := strconv.ParseFloat(d.value, 64)
			perfMetrics = append(perfMetrics, &CheckMetric{
				Name:  d.key,
				Value: value,
			})
		}
	}

	// compare ram metrics to tresholds
	if CompareMetrics(mdata, warnTreshold) {
		state = CheckExitWarning
	}

	if CompareMetrics(mdata, critTreshold) {
		state = CheckExitCritical
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: perfMetrics,
	}, nil
}
