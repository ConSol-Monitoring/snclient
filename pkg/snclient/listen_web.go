package snclient

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
)

func init() {
	RegisterModule(
		&AvailableListeners,
		"WEBServer",
		"/settings/WEB/server",
		NewHandlerWeb,
		ConfigInit{
			ConfigData{
				"port":                   "8443",
				"use ssl":                "1",
				"allow arguments":        "true",
				"allow nasty characters": "false",
			},
			"/settings/default",
			DefaultListenHTTPConfig,
		},
	)
}

const MaxHTTPHeaderTimeoutOverride = 5 * time.Minute

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

type CheckWebPerfNumber string

func (n CheckWebPerfNumber) MarshalJSON() ([]byte, error) {
	if n == "U" {
		return []byte(`"U"`), nil
	}

	return []byte(n), nil
}

type HandlerWeb struct {
	noCopy          noCopy
	handlerGeneric  http.Handler
	handlerLegacy   http.Handler
	handlerV1       http.Handler
	conf            *ConfigSection
	password        string
	requirePassword bool
	snc             *Agent
	listener        *Listener
	allowedHosts    *AllowedHostConfig
}

// ensure we fully implement the RequestHandlerHTTP type
var _ RequestHandlerHTTP = &HandlerWeb{}

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

func (l *HandlerWeb) Listener() *Listener {
	return l.listener
}

func (l *HandlerWeb) Start() error {
	return l.listener.Start()
}

func (l *HandlerWeb) Stop() {
	if l.listener != nil {
		l.listener.Stop()
	}
}

func (l *HandlerWeb) Init(snc *Agent, conf *ConfigSection, _ *Config, runSet *AgentRunSet) error {
	l.snc = snc
	l.conf = conf

	err := setListenerAuthInit(&l.password, &l.requirePassword, conf)
	if err != nil {
		return err
	}

	listener, err := SharedWebListener(snc, conf, l, runSet)
	if err != nil {
		return err
	}
	l.listener = listener

	allowedHosts, err := NewAllowedHostConfig(conf)
	if err != nil {
		return err
	}
	l.allowedHosts = allowedHosts

	return nil
}

func (l *HandlerWeb) GetAllowedHosts() *AllowedHostConfig {
	return l.allowedHosts
}

func (l *HandlerWeb) CheckPassword(req *http.Request, _ URLMapping) bool {
	switch req.URL.Path {
	case "/", "/index.html":
		return true
	default:
		return verifyRequestPassword(l.snc, req, l.password, l.requirePassword)
	}
}

func (l *HandlerWeb) GetMappings(*Agent) []URLMapping {
	return []URLMapping{
		{URL: "/query/{command}", Handler: l.handlerLegacy},
		{URL: "/api/v1/queries/{command}/commands/execute", Handler: l.handlerV1},
		{URL: "/api/v1/inventory", Handler: l.handlerV1},
		{URL: "/api/v1/inventory/", Handler: l.handlerV1},
		{URL: "/api/v1/inventory/{module}", Handler: l.handlerV1},
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
		perf := l.metric2Perf(metric)
		if metric.PerfConfig != nil && metric.PerfConfig.Ignore {
			continue
		}
		result = append(result, perf)
	}

	return result
}

func (l *HandlerWeb) metric2Perf(metric *CheckMetric) CheckWebPerf {
	perf := CheckWebPerf{
		Alias: metric.tweakedName(),
	}
	numStr, unit := metric.tweakedNum(metric.Value)
	val := CheckWebPerfVal{
		Value: CheckWebPerfNumber(numStr),
		Unit:  unit,
	}

	if metric.Warning != nil {
		warn := metric.ThresholdString(metric.Warning)
		val.Warning = &warn
	}
	if metric.Critical != nil {
		crit := metric.ThresholdString(metric.Critical)
		val.Critical = &crit
	}
	if utils.IsFloatVal(numStr) {
		l.metrics2PerfFloatMinMax(metric, &val)
		perf.FloatVal = &val
	} else {
		l.metrics2PerfInt64MinMax(metric, &val)
		perf.IntVal = &val
	}

	return perf
}

func (l *HandlerWeb) metrics2PerfFloatMinMax(metric *CheckMetric, val *CheckWebPerfVal) {
	if metric.PerfConfig != nil {
		if metric.Min != nil {
			num, _ := metric.tweakedNum(*metric.Min)
			val.Min = CheckWebPerfNumber(num)
		}

		if metric.Max != nil {
			num, _ := metric.tweakedNum(*metric.Max)
			val.Max = CheckWebPerfNumber(num)
		}

		return
	}

	if metric.Min != nil {
		val.Min = metric.Min
	}
	if metric.Max != nil {
		val.Max = metric.Max
	}
}

func (l *HandlerWeb) metrics2PerfInt64MinMax(metric *CheckMetric, val *CheckWebPerfVal) {
	if metric.PerfConfig != nil {
		if metric.Min != nil {
			num, _ := metric.tweakedNum(*metric.Min)
			val.Min = CheckWebPerfNumber(num)
		}

		if metric.Max != nil {
			num, _ := metric.tweakedNum(*metric.Max)
			val.Max = CheckWebPerfNumber(num)
		}

		return
	}

	if metric.Min != nil {
		minV := int64(*metric.Min)
		val.Min = &minV
	}

	if metric.Max != nil {
		maxV := int64(*metric.Max)
		val.Max = &maxV
	}
}

// return "lines" list suitable as v1 result, each metric will be a new line to keep the order of the metrics
func (l *HandlerWeb) result2V1(result *CheckResult) (v1Res []CheckWebLineV1) {
	v1Res = []CheckWebLineV1{{
		Message: result.Output,
		Perf:    map[string]interface{}{},
	}}
	if len(result.Metrics) == 0 {
		return v1Res
	}
	for idx, metric := range result.Metrics {
		perfRes := make(map[string]interface{}, 0)
		if metric.PerfConfig != nil && metric.PerfConfig.Ignore {
			continue
		}
		perfData := l.metric2Perf(metric)
		val := perfData.FloatVal
		if val == nil {
			val = perfData.IntVal
		}
		perf := map[string]interface{}{
			"value": val.Value,
			"unit":  val.Unit,
		}
		if val.Warning != nil {
			perf["warning"] = *val.Warning
		}
		if val.Critical != nil {
			perf["critical"] = *val.Critical
		}
		if val.Min != nil {
			perf["minimum"] = val.Min
		}
		if val.Max != nil {
			perf["maximum"] = val.Max
		}

		// first metric goes into first row, others append to a new line
		perfRes[perfData.Alias] = perf
		if idx == 0 {
			v1Res[0].Perf = perfRes
		} else {
			v1Res = append(v1Res, CheckWebLineV1{
				Message: "",
				Perf:    perfRes,
			})
		}
	}

	return v1Res
}

func verifyRequestPassword(snc *Agent, req *http.Request, requiredPassword string, requirePassword bool) bool {
	if !requirePassword {
		return true
	}
	// check basic auth password
	_, password, _ := req.BasicAuth()
	if password == "" {
		// fallback to clear text  password from http header
		password = req.Header.Get("Password")
	}

	return snc.verifyPassword(requiredPassword, password)
}

// runCheck calls check by name and returns the check result
func (l *HandlerWeb) runCheck(req *http.Request, command string) (result *CheckResult) {
	args := queryParam2CommandArgs(req)

	// extend timeout from check_nsc_web
	timeoutSeconds := float64(0)
	timeout := req.Header.Get("X-Nsc-Web-Timeout")
	if timeout != "" {
		dur, err := utils.ExpandDuration(timeout)
		if err == nil {
			if dur > DefaultCheckTimeout.Seconds() && dur <= MaxHTTPHeaderTimeoutOverride.Seconds() {
				timeoutSeconds = dur
				log.Tracef("extended timeout from http header: %s", time.Duration(dur*float64(time.Second)).String())
			}
		} else {
			log.Debugf("failed to parse timeout: %s", err.Error())
		}
	}

	return l.snc.RunCheckWithContext(req.Context(), command, args, timeoutSeconds, l.conf)
}

type HandlerWebLegacy struct {
	noCopy  noCopy
	Handler *HandlerWeb
}

func (l *HandlerWebLegacy) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	command := chi.URLParam(req, "command")
	result := l.Handler.runCheck(req, command)
	jsonData := map[string]interface{}{
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
	}
	data, err := json.Marshal(jsonData)
	if err != nil {
		msg := fmt.Sprintf("json error: %s", err.Error())
		log.Errorf("%s", msg)
		res.WriteHeader(http.StatusInternalServerError)
		LogError2(res.Write([]byte(msg)))

		return
	}
	res.WriteHeader(http.StatusOK)
	res.Header().Set("Content-Type", "application/json")
	LogError2(res.Write(data))

	if log.IsV(2) {
		log.Tracef("sending legacy json result:")
		log.Tracef("<<<<")
		log.Tracef("%s", data)
		log.Tracef(">>>>")
	}
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
		LogError2(res.Write([]byte("404 - nothing here\n")))
	}
}

type HandlerWebV1 struct {
	noCopy  noCopy
	Handler *HandlerWeb
}

func (l *HandlerWebV1) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	path := strings.TrimSuffix(req.URL.Path, "/")
	switch {
	case strings.HasPrefix(path, "/api/v1/inventory"):
		l.serveInventory(res, req)
	default:
		l.serveCommand(res, req)
	}
}

func (l *HandlerWebV1) serveCommand(res http.ResponseWriter, req *http.Request) {
	command := chi.URLParam(req, "command")
	result := l.Handler.runCheck(req, command)
	res.Header().Set("Content-Type", "application/json")
	jsonData := map[string]interface{}{
		"command": command,
		"result":  result.State,
		"lines":   l.Handler.result2V1(result),
	}
	data, err := json.Marshal(jsonData)
	if err != nil {
		msg := fmt.Sprintf("json error: %s", err.Error())
		log.Errorf("%s", msg)
		res.WriteHeader(http.StatusInternalServerError)
		LogError2(res.Write([]byte(msg)))

		return
	}
	res.WriteHeader(http.StatusOK)
	LogError2(res.Write(data))

	if log.IsV(2) {
		log.Tracef("sending v1 json result:")
		log.Tracef("<<<<")
		log.Tracef("%s", data)
		log.Tracef(">>>>")
	}
}

func (l *HandlerWebV1) serveInventory(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	module := chi.URLParam(req, "module")
	var modules []string
	if module != "" {
		modules = []string{module}
	}
	inventory := l.Handler.snc.GetInventory(req.Context(), modules)

	LogError(json.NewEncoder(res).Encode(inventory))
}
