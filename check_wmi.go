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
	data   CheckData
}

type WMI struct {
	key   string
	value string
}

func QueryWMI(query string) (querydata []WMI, out string) {
	var ret []WMI
	var output []string

	err := ole.CoInitialize(0)
	if err != nil {
		log.Debugf("check_service: couldnt initialize COM connection: %s\n", err)
	}
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
		func() {
			itemRaw, _ := oleutil.CallMethod(result, "ItemIndex", i)
			item := itemRaw.ToIDispatch()
			defer item.Release()

			for _, v := range values {
				s, _ := oleutil.GetProperty(item, strings.TrimSpace(v))
				ret = append(ret, WMI{key: v, value: s.ToString()})
				output = append(output, s.ToString())
			}
		}()
	}

	return ret, strings.Join(output, ", ")
}

/* check_wmi
 * Description: Querys the WMI for several metrics.
 * Tresholds: keys of the query
 * Units: none
 */
func (l *CheckWMI) Check(args []string) (*CheckResult, error) {
	// default state: OK
	state := int64(0)
	argList := ParseArgs(args, &l.data)
	var query string

	// parse treshold args
	for _, arg := range argList {
		switch arg.key {
		case "query":
			query = arg.value
		default:
			log.Debugf("unknown argument: %s", arg.key)
		}
	}

	// query wmi
	querydata, output := QueryWMI(query)

	mdata := []MetricData{}
	perfMetrics := []*CheckMetric{}

	for _, d := range querydata {
		mdata = append(mdata, MetricData{name: d.key, value: d.value})
		if d.key == l.data.warnTreshold.name || d.key == l.data.critTreshold.name {
			value, _ := strconv.ParseFloat(d.value, 64)
			perfMetrics = append(perfMetrics, &CheckMetric{
				Name:  d.key,
				Value: value,
			})
		}
	}

	// compare metrics to tresholds
	if CompareMetrics(mdata, l.data.warnTreshold) {
		state = CheckExitWarning
	}

	if CompareMetrics(mdata, l.data.critTreshold) {
		state = CheckExitCritical
	}

	return &CheckResult{
		State:   state,
		Output:  output,
		Metrics: perfMetrics,
	}, nil
}
