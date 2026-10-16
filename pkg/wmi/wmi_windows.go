package wmi

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/kdar/factorlog"
	ywmi "github.com/yusufpapurcu/wmi"
)

// wmiWorker is a persistent WMI connection. It wraps the opaque
// ywmi.SWBemServices type behind method values so it never has to be named.
type wmiWorker struct {
	query func(query string, dst any, args ...any) error
	close func() error
}

// newWmiWorker creates a persistent WMI connection. Reusing a connection
// across queries prevents a memory leak on WMF 5+. See
// https://github.com/prometheus-community/windows_exporter/issues/77 and
// linked issues for details.
func newWmiWorker() (wmiWorker, error) {
	s, err := ywmi.InitializeSWbemServices(&ywmi.Client{AllowMissingFields: true})
	if err != nil {
		return wmiWorker{}, fmt.Errorf("InitializeSWbemServices: %s", err.Error())
	}

	return wmiWorker{query: s.Query, close: s.Close}, nil
}

var (
	pool     chan *wmiWorker
	debugLog *factorlog.FactorLog
)

// InitWbem initializes the WMI backend. poolSize is how many queries may run
// in parallel: <= 1 reuses the single shared default client, > 1 starts a
// fixed pool of that many persistent workers.
func InitWbem(log *factorlog.FactorLog, poolSize int) error {
	debugLog = log
	if poolSize <= 1 {
		s, err := ywmi.InitializeSWbemServices(ywmi.DefaultClient)
		if err != nil {
			return fmt.Errorf("InitializeSWbemServices: %s", err.Error())
		}
		ywmi.DefaultClient.AllowMissingFields = true
		ywmi.DefaultClient.SWbemServicesClient = s

		return nil
	}

	workers := make([]wmiWorker, 0, poolSize)
	for range poolSize {
		worker, err := newWmiWorker()
		if err != nil {
			for j := range workers {
				_ = workers[j].close()
			}

			return err
		}
		workers = append(workers, worker)
	}
	pool = make(chan *wmiWorker, poolSize)
	for i := range workers {
		pool <- &workers[i]
	}

	return nil
}

// QueryDefaultRetry sends a query with the default retry duration of 1sec
func QueryDefaultRetry(query string, dst any) (err error) {
	return QueryWithRetries(query, dst, "", 3, 1*time.Second)
}

// QueryWithRetries executes the given query with specified retries and delay between retries.
func QueryWithRetries(query string, dst any, namespace string, retries int, delay time.Duration) (err error) {
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
func Query(query string, dst any, namespace string) (err error) {
	query = strings.TrimSpace(query)
	// MS_409 equals en_US, see https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-wmi/259edd31-d6eb-4bc9-a2c4-2891b78bb51d
	// connect parameters are here: https://learn.microsoft.com/en-us/windows/win32/wmisdk/swbemlocator-connectserver
	var args []any
	if namespace != "" {
		args = []any{nil, namespace, nil, nil, "MS_409"}
	} else {
		args = []any{nil, nil, nil, nil, "MS_409"}
	}

	if pool == nil {
		t2 := time.Now()
		err = ywmi.Query(query, dst, args...)
		if debugLog != nil {
			debugLog.Debugf("wmi query took %s", time.Since(t2))
		}
	} else {
		t1 := time.Now()
		worker := <-pool
		defer func() { pool <- worker }()
		if debugLog != nil {
			debugLog.Debugf("wmi query waited %s for a pooled client: %s", time.Since(t1), query)
		}
		t2 := time.Now()
		err = worker.query(query, dst, args...)
		if debugLog != nil {
			debugLog.Debugf("wmi query took %s", time.Since(t2))
		}
	}

	if err != nil {
		return fmt.Errorf("ywmi query failed: %s", err.Error())
	}

	return nil
}

// RawQuery sends a query and returns a 2dimensional data array or any error encountered.
// The namespace is optional and can be empty.
func RawQuery(query, namespace string) (res [][]Data, err error) {
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

	// service is a SWbemServices. ConnectServer args are [server, namespace, ...];
	// nil keeps the local machine and default namespace.
	var serviceRaw *ole.VARIANT
	if namespace != "" {
		serviceRaw, err = oleutil.CallMethod(wmi, "ConnectServer", nil, namespace)
	} else {
		serviceRaw, err = oleutil.CallMethod(wmi, "ConnectServer")
	}
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
