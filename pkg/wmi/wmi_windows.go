package wmi

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	ywmi "github.com/yusufpapurcu/wmi"
)

func InitWbem() error {
	// This initialization prevents a memory leak on WMF 5+. See
	// https://github.com/prometheus-community/windows_exporter/issues/77 and
	// linked issues for details.
	s, err := ywmi.InitializeSWbemServices(ywmi.DefaultClient)
	if err != nil {
		return fmt.Errorf("InitializeSWbemServices: %s", err.Error())
	}
	ywmi.DefaultClient.AllowMissingFields = true
	ywmi.DefaultClient.SWbemServicesClient = s

	return nil
}

// QueryDefaultRetry sends a query with the default retry duration of 1sec
func QueryDefaultRetry(query string, dst interface{}) (err error) {
	return QueryWithRetries(query, dst, "", 3, 1*time.Second)
}

// QueryWithRetries executes the given query with specified retries and delay between retries.
func QueryWithRetries(query string, dst interface{}, namespace string, retries int, delay time.Duration) (err error) {
	for retries > 0 {
		err = Query(query, dst, namespace)
		if err == nil {
			return nil
		}
		retries--
		if retries == 0 {
			break
		}
		time.Sleep(delay)
	}

	return err
}

// Query sends a single query, the namespace is optional and can be empty
func Query(query string, dst interface{}, namespace string) (err error) {
	query = strings.TrimSpace(query)
	// MS_409 equals en_US, see https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-wmi/259edd31-d6eb-4bc9-a2c4-2891b78bb51d
	// connect parameters are here: https://learn.microsoft.com/en-us/windows/win32/wmisdk/swbemlocator-connectserver
	if namespace != "" {
		err = ywmi.Query(query, dst, nil, namespace, nil, nil, "MS_409")
	} else {
		err = ywmi.Query(query, dst, nil, nil, nil, nil, "MS_409")
	}

	return err
}

// RawQuery sends a query and returns a 2dimensional data array or any error encountered
func RawQuery(query string) (res [][]Data, err error) {
	query = strings.TrimSpace(query)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err = ole.CoInitialize(0)
	if err != nil {
		return nil, fmt.Errorf("wmi: ole.CoInitialize failed: %s", err.Error())
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return nil, fmt.Errorf("wmi: CreateObject WbemScripting.SWbemLocator failed: %s", err.Error())
	}
	defer unknown.Release()

	wmi, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("wmi: QueryInterface ole.IID_IDispatch failed: %s", err.Error())
	}
	defer wmi.Release()

	// service is a SWbemServices
	serviceRaw, err := oleutil.CallMethod(wmi, "ConnectServer")
	if err != nil {
		return nil, fmt.Errorf("wmi: CallMethod ConnectServer failed: %s", err.Error())
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()

	// result is a SWBemObjectSet
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery", query)
	if err != nil {
		return nil, fmt.Errorf("wmi: CallMethod ExecQuery failed: %s", err.Error())
	}
	result := resultRaw.ToIDispatch()
	defer result.Release()

	countVar, _ := oleutil.GetProperty(result, "Count")
	count := countVar.Val

	re := regexp.MustCompile(`\w+\s+((?:\w+\s*,\s*)*\w+)`)
	values := strings.Split(re.FindStringSubmatch(query)[1], ",")

	ret := make([][]Data, 0)
	for i := range count {
		// item is a SWbemObject, but really a Win32_Process
		obj, err := processResult(values, result, i)
		if err != nil {
			return nil, err
		}
		ret = append(ret, obj)
	}

	return ret, nil
}

func processResult(values []string, result *ole.IDispatch, i int64) (obj []Data, err error) {
	itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
	if err != nil {
		return nil, fmt.Errorf("oleutil.CallMethod failed: %s", err.Error())
	}
	item := itemRaw.ToIDispatch()
	defer item.Release()

	for _, val := range values {
		var value string
		property, err := oleutil.GetProperty(item, strings.TrimSpace(val))
		if err != nil {
			return nil, fmt.Errorf("oleutil.GetProperty failed for item: %s (%s)", val, err.Error())
		}
		if property.Value() == nil {
			value = ""
		} else {
			switch t := property.Value().(type) {
			case int32:
				value = fmt.Sprintf("%d", t)
			default:
				value = property.ToString()
			}
		}
		obj = append(obj, Data{Key: strings.TrimSpace(val), Value: value})
	}

	return obj, nil
}
