package wmi

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

type Data struct {
	Key   string
	Value string
}

func Query(query string) (querydata [][]Data, err error) {
	var ret [][]Data

	err = ole.CoInitialize(0)
	if err != nil {
		return nil, fmt.Errorf("check_service: couldn't initialize COM connection: %s", err.Error())
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
		err = func() error {
			itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
			if err != nil {
				return fmt.Errorf("wmi call failed: %s", err.Error())
			}
			item := itemRaw.ToIDispatch()
			defer item.Release()

			var obj []Data

			for _, val := range values {
				var value string
				property, err := oleutil.GetProperty(item, strings.TrimSpace(val))
				if err != nil {
					return fmt.Errorf("WMI: error getting property from item (%s)", err.Error())
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

			ret = append(ret, obj)

			return nil
		}()
		if err != nil {
			return nil, fmt.Errorf("wmi error: %s", err.Error())
		}
	}

	return ret, nil
}

func ResultToMap(queryResult [][]Data) []map[string]string {
	ret := make([]map[string]string, 0, len(queryResult))

	for _, result := range queryResult {
		mapObj := make(map[string]string, len(result))

		for _, obj := range result {
			mapObj[obj.Key] = obj.Value
		}

		ret = append(ret, mapObj)
	}

	return ret
}
