package wmi

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	ywmi "github.com/yusufpapurcu/wmi"
)

func QueryDefaultRetry(query string, dst interface{}) (err error) {
	return QueryWithRetries(query, dst, 3, 1*time.Second)
}

func QueryWithRetries(query string, dst interface{}, retries int, delay time.Duration) (err error) {
	for retries > 0 {
		err = Query(query, dst)
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

func Query(query string, dst interface{}) (err error) {
	query = strings.TrimSpace(query)
	err = ywmi.Query(query, dst)

	return
}

func RawQuery(query string) (res [][]Data, err error) {
	query = strings.TrimSpace(query)
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
	for i := int64(0); i < count; i++ {
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
