package wmi

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

type WMIData struct {
	Key   string
	Value string
}

func Query(query string) (querydata [][]WMIData, out string) {
	var ret [][]WMIData
	var output []string

	err := ole.CoInitialize(0)
	if err != nil {
		fmt.Printf("check_service: couldnt initialize COM connection: %s\n", err)
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
			itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
			if err != nil {
				return
			}
			item := itemRaw.ToIDispatch()
			defer item.Release()

			var obj []WMIData

			for _, v := range values {
				var value string
				s, err := oleutil.GetProperty(item, strings.TrimSpace(v))
				if err != nil {
					fmt.Printf("WMI: error getting property from item (%v)\n", err)
					continue
				}
				if s.Value() == nil {
					value = ""
				} else {
					if reflect.TypeOf(s.Value()).String() == "int32" {
						value = strconv.FormatInt(int64(s.Value().(int32)), 10)
					} else {
						value = s.ToString()
					}
				}
				obj = append(obj, WMIData{Key: strings.TrimSpace(v), Value: value})
				output = append(output, s.ToString())
			}

			ret = append(ret, obj)

		}()
	}

	return ret, strings.Join(output, ", ")
}

func ResultToMap(queryResult [][]WMIData) []map[string]string {

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
