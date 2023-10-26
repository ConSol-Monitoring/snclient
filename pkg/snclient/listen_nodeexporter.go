package snclient

import (
	"fmt"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"pkg/humanize"
	"pkg/utils"

	"github.com/shirou/gopsutil/v3/process"
)

func init() {
	RegisterModule(&AvailableListeners, "NodeExporterServer", "/settings/NodeExporter/server", NewHandlerNodeExporter)
}

const (
	nodeExporterRestartDelay     = 3 * time.Second
	nodeExporterMemWatchInterval = 30 * time.Second
)

type HandlerNodeExporter struct {
	agentPath    string
	agentArgs    string
	agentAddress string
	agentMaxMem  uint64
	cmd          *exec.Cmd
	pid          int
	snc          *Agent
	keepRunningA atomic.Bool
	password     string
	urlPrefix    string
	listener     *Listener
	proxy        *httputil.ReverseProxy
}

func NewHandlerNodeExporter() Module {
	l := &HandlerNodeExporter{}

	return l
}

func (l *HandlerNodeExporter) Type() string {
	return "nodeexporter"
}

func (l *HandlerNodeExporter) BindString() string {
	return l.listener.BindString()
}

func (l *HandlerNodeExporter) Listener() *Listener {
	return l.listener
}

func (l *HandlerNodeExporter) Start() error {
	l.keepRunningA.Store(true)
	go func() {
		defer l.snc.logPanicExit()
		l.procMainLoop()
	}()

	return l.listener.Start()
}

func (l *HandlerNodeExporter) Stop() {
	l.keepRunningA.Store(false)
	l.listener.Stop()
	l.StopProc()
}

func (l *HandlerNodeExporter) StopProc() {
	if l.cmd != nil && l.cmd.Process != nil {
		LogError(l.cmd.Process.Kill())
	}
	l.cmd = nil
	l.pid = 0
}

func (l *HandlerNodeExporter) Defaults() ConfigData {
	defaults := ConfigData{
		"port":             "8443",
		"agent address":    "127.0.0.1:9999",
		"agent max memory": "256M",
		"use ssl":          "1",
		"url prefix":       "/node",
	}
	defaults.Merge(DefaultListenHTTPConfig)

	return defaults
}

func (l *HandlerNodeExporter) Init(snc *Agent, conf *ConfigSection, _ *Config, set *ModuleSet) error {
	l.snc = snc

	l.password = DefaultPassword
	if password, ok := conf.GetString("password"); ok {
		l.password = password
	}

	listener, err := SharedWebListener(snc, conf, l, set)
	if err != nil {
		return err
	}
	l.listener = listener
	urlPrefix, _ := conf.GetString("url prefix")
	l.urlPrefix = strings.TrimSuffix(urlPrefix, "/")

	if agentPath, ok := conf.GetString("agent path"); ok {
		l.agentPath = agentPath
	}
	if l.agentPath == "" {
		return fmt.Errorf("agent path is required to start the node agent")
	}

	if agentArgs, ok := conf.GetString("agent args"); ok {
		l.agentArgs = agentArgs
	}

	if agentMaxMem, ok := conf.GetString("agent max memory"); ok {
		maxMem, err := humanize.ParseBytes(agentMaxMem)
		if err != nil {
			return fmt.Errorf("agent max memory: %s", err.Error())
		}
		l.agentMaxMem = maxMem
	}

	if agentAddress, ok := conf.GetString("agent address"); ok {
		l.agentAddress = agentAddress
	}

	uri, err := url.Parse("http://" + l.agentAddress + "/metrics")
	if err != nil {
		return fmt.Errorf("cannot parse agent url: %s", err.Error())
	}

	l.proxy = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL = uri
		},
		ErrorHandler: getReverseProxyErrorHandlerFunc(l.Type()),
	}

	return nil
}

func (l *HandlerNodeExporter) GetMappings(*Agent) []URLMapping {
	return []URLMapping{
		{URL: l.urlPrefix + "/metrics", Handler: l.proxy},
	}
}

func (l *HandlerNodeExporter) keepRunning() bool {
	return l.keepRunningA.Load()
}

func (l *HandlerNodeExporter) procMainLoop() {
	for l.keepRunning() {
		args := utils.Tokenize(l.agentArgs)
		if len(args) == 1 && args[0] == "" {
			args = []string{}
		}
		args = append(args, fmt.Sprintf("--web.listen-address=%s", l.agentAddress))
		cmd := exec.Command(l.agentPath, args...) //nolint:gosec // input source is the config file
		log.Debugf("starting node agent: %s", cmd.Path)
		l.snc.passthroughLogs("stdout", "["+l.Type()+"] ", log.Debugf, cmd.StdoutPipe)
		l.snc.passthroughLogs("stderr", "["+l.Type()+"] ", l.logPass, cmd.StderrPipe)

		err := cmd.Start()
		if err != nil {
			err = fmt.Errorf("failed to start node agent: %w: %s", err, err.Error())
			log.Errorf("node agent startup error: %s", err)

			return
		}

		l.pid = cmd.Process.Pid
		l.cmd = cmd

		go func() {
			defer l.snc.logPanicExit()

			l.procMemWatcher()
		}()

		err = cmd.Wait()
		if !l.keepRunning() {
			return
		}
		if err != nil {
			log.Errorf("node agent errored: %s", err.Error())

			time.Sleep(nodeExporterRestartDelay)
		}
	}
}

func (l *HandlerNodeExporter) procMemWatcher() {
	ticker := time.NewTicker(nodeExporterMemWatchInterval)
	defer ticker.Stop()

	for {
		<-ticker.C
		if !l.keepRunning() {
			return
		}
		if l.cmd == nil {
			return
		}
		proc, err := process.NewProcess(int32(l.pid))
		if err != nil {
			log.Debugf("failed to get process: %s", err.Error())

			return
		}

		memInfo, err := proc.MemoryInfo()
		if err != nil {
			log.Debugf("failed to get process memory: %s", err.Error())

			return
		}

		if memInfo.RSS > l.agentMaxMem {
			log.Warnf("nodeexporter memory usage - rss: %s, vms: %s -> restarting the process", humanize.BytesF(memInfo.RSS, 2), humanize.BytesF(memInfo.VMS, 2))
			l.StopProc()
		} else {
			log.Tracef("nodeexporter memory usage - rss: %s, vms: %s", humanize.BytesF(memInfo.RSS, 2), humanize.BytesF(memInfo.VMS, 2))
		}
	}
}

func (l *HandlerNodeExporter) logPass(f string, v ...interface{}) {
	entry := fmt.Sprintf(f, v...)
	switch {
	case strings.Contains(entry, "level=info"):
		log.Debug(entry)
	case strings.Contains(entry, "level=debug"):
		log.Trace(entry)
	default:
		log.Error(entry)
	}
}
