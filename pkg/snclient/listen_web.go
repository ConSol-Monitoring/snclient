package snclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"pkg/convert"
	"pkg/utils"

	"github.com/go-chi/chi/v5"
)

const (
	// DefaultPassword sets default password, login with default password is not
	// possible. It needs to be changed in the ini file.
	DefaultPassword = "CHANGEME"
)

type CheckWebLine struct {
	Message string         `json:"message"`
	Perf    []CheckWebPerf `json:"perf,omitempty"`
}

type CheckWebLineV1 struct {
	Message string                 `json:"message"`
	Perf    map[string]interface{} `json:"perf,omitempty"`
}

type CheckWebPerf struct {
	Alias    string           `json:"alias"`
	IntVal   *CheckWebPerfVal `json:"int_value,omitempty"`
	FloatVal *CheckWebPerfVal `json:"float_value,omitempty"`
}

type CheckWebPerfVal struct {
	Value    CheckWebPerfNumber `json:"value"`
	Unit     string             `json:"unit"`
	Min      interface{}        `json:"minimum,omitempty"`
	Max      interface{}        `json:"maximum,omitempty"`
	Warning  *string            `json:"warning,omitempty"`
	Critical *string            `json:"critical,omitempty"`
}

type CheckWebPerfNumber struct {
	num interface{}
}

func (n CheckWebPerfNumber) MarshalJSON() ([]byte, error) {
	if fmt.Sprintf("%v", n.num) == "U" {
		return []byte(`"U"`), nil
	}
	val, err := convert.Num2StringE(n.num)
	if err != nil {
		return nil, fmt.Errorf("num2string: %s", err.Error())
	}

	return []byte(val), nil
}

func init() {
	RegisterModule(&AvailableListeners, "WEBServer", "/settings/WEB/server", NewHandlerWeb)
}

type HandlerWeb struct {
	noCopy         noCopy
	handlerGeneric http.Handler
	handlerLegacy  http.Handler
	handlerV1      http.Handler
	password       string
	snc            *Agent
	listener       *Listener
}

func NewHandlerWeb() Module {
	l := &HandlerWeb{}
	l.handlerGeneric = &HandlerWebGeneric{Handler: l}
	l.handlerLegacy = &HandlerWebLegacy{Handler: l}
	l.handlerV1 = &HandlerWebV1{Handler: l}

	return l
}

func (l *HandlerWeb) Type() string {
	if l.listener != nil && l.listener.tlsConfig != nil {
		return "https"
	}

	return "http"
}

func (l *HandlerWeb) BindString() string {
	return l.listener.BindString()
}

func (l *HandlerWeb) Start() error {
	return l.listener.Start()
}

func (l *HandlerWeb) Stop() {
	if l.listener != nil {
		l.listener.Stop()
	}
}

func (l *HandlerWeb) Defaults() ConfigData {
	defaults := ConfigData{
		"port":                   "8443",
		"use ssl":                "1",
		"password":               DefaultPassword,
		"allow arguments":        "true",
		"allow nasty characters": "false",
	}
	defaults.Merge(DefaultListenHTTPConfig)

	return defaults
}

func (l *HandlerWeb) Init(snc *Agent, conf *ConfigSection, _ *Config) error {
	l.snc = snc
	if password, ok := conf.GetString("password"); ok {
		l.password = password
	}

	listener, err := NewListener(snc, conf, l)
	if err != nil {
		return err
	}
	l.listener = listener

	return nil
}

func (l *HandlerWeb) GetMappings(*Agent) []URLMapping {
	return []URLMapping{
		{URL: "/query/{command}", Handler: l.handlerLegacy},
		{URL: "/api/v1/queries/{command}/commands/execute", Handler: l.handlerV1},
		{URL: "/index.html", Handler: l.handlerGeneric},
		{URL: "/", Handler: l.handlerGeneric},
	}
}

func queryParam2CommandArgs(req *http.Request) []string {
	args := make([]string, 0)

	query := req.URL.RawQuery
	if query == "" {
		return args
	}

	for _, v := range strings.Split(query, "&") {
		u, _ := url.QueryUnescape(v)
		args = append(args, u)
	}

	return args
}

func (l *HandlerWeb) metrics2Perf(metrics []*CheckMetric) []CheckWebPerf {
	if len(metrics) == 0 {
		return nil
	}
	result := make([]CheckWebPerf, 0)

	for _, metric := range metrics {
		perf := CheckWebPerf{
			Alias: metric.Name,
		}
		val := CheckWebPerfVal{
			Unit: metric.Unit,
		}
		if metric.Warning != nil {
			warn := metric.ThresholdString(metric.Warning)
			val.Warning = &warn
		}
		if metric.Critical != nil {
			crit := metric.ThresholdString(metric.Critical)
			val.Critical = &crit
		}
		if utils.IsFloatVal(metric.Value) {
			val.Min = metric.Min
			val.Max = metric.Max
			val.Value = CheckWebPerfNumber{num: metric.Value}
			perf.FloatVal = &val
		} else {
			if metric.Min != nil {
				min := int64(*metric.Min)
				val.Min = &min
			}
			if metric.Max != nil {
				max := int64(*metric.Max)
				val.Max = &max
			}
			val.Value = CheckWebPerfNumber{num: metric.Value}
			perf.IntVal = &val
		}
		result = append(result, perf)
	}

	return result
}

func (l *HandlerWeb) metrics2PerfV1(metrics []*CheckMetric) map[string]interface{} {
	if len(metrics) == 0 {
		return nil
	}
	result := make(map[string]interface{}, 0)

	for _, metric := range metrics {
		perf := map[string]interface{}{
			"value": CheckWebPerfNumber{num: metric.Value},
			"unit":  metric.Unit,
		}
		if metric.Warning != nil {
			perf["warning"] = metric.ThresholdString(metric.Warning)
		}
		if metric.Critical != nil {
			perf["critical"] = metric.ThresholdString(metric.Critical)
		}
		if metric.Min != nil {
			perf["minimum"] = *metric.Min
		}
		if metric.Max != nil {
			perf["maximum"] = *metric.Max
		}
		result[metric.Name] = perf
	}

	return result
}

type HandlerWebLegacy struct {
	noCopy  noCopy
	Handler *HandlerWeb
}

func (l *HandlerWebLegacy) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	// check clear text password
	if !l.Handler.snc.verifyPassword(l.Handler.password, req.Header.Get("Password")) {
		http.Error(res, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		res.Header().Set("Content-Type", "application/json")
		LogError(json.NewEncoder(res).Encode(map[string]interface{}{
			"error": "permission denied",
		}))

		return
	}

	command := chi.URLParam(req, "command")
	args := queryParam2CommandArgs(req)
	result := l.Handler.snc.RunCheck(command, args)
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	LogError(json.NewEncoder(res).Encode(map[string]interface{}{
		"payload": []interface{}{
			map[string]interface{}{
				"command": command,
				"result":  result.StateString(),
				"lines": []CheckWebLine{
					{
						Message: result.Output,
						Perf:    l.Handler.metrics2Perf(result.Metrics),
					},
				},
			},
		},
	}))
}

type HandlerWebGeneric struct {
	noCopy  noCopy
	Handler *HandlerWeb
}

func (l *HandlerWebGeneric) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/":
		http.Redirect(res, req, "/index.html", http.StatusTemporaryRedirect)
	case "/index.html":
		res.WriteHeader(http.StatusOK)
		LogError2(res.Write([]byte("snclient working...")))
	default:
		res.WriteHeader(http.StatusNotFound)
		LogError2(res.Write([]byte("404 - nothing here")))
	}
}

type HandlerWebV1 struct {
	noCopy  noCopy
	Handler *HandlerWeb
}

func (l *HandlerWebV1) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	// check basic auth password
	_, password, _ := req.BasicAuth()
	if !l.Handler.snc.verifyPassword(l.Handler.password, password) {
		http.Error(res, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		res.Header().Set("Content-Type", "application/json")
		LogError(json.NewEncoder(res).Encode(map[string]interface{}{
			"error": "permission denied",
		}))

		return
	}

	command := chi.URLParam(req, "command")
	args := queryParam2CommandArgs(req)
	result := l.Handler.snc.RunCheck(command, args)
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	LogError(json.NewEncoder(res).Encode(map[string]interface{}{
		"command": command,
		"result":  result.State,
		"lines": []CheckWebLineV1{
			{
				Message: result.Output,
				Perf:    l.Handler.metrics2PerfV1(result.Metrics),
			},
		},
	}))
}
