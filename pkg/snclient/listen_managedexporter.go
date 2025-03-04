package snclient

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/consol-monitoring/snclient/pkg/convert"
	"github.com/consol-monitoring/snclient/pkg/humanize"
	"github.com/consol-monitoring/snclient/pkg/utils"
	"github.com/shirou/gopsutil/v4/process"
)

const (
	managedExporterRestartDelay     = 3 * time.Second
	managedExporterMemWatchInterval = 30 * time.Second
)

type HandlerManagedExporter struct {
	name           string
	agentPath      string
	agentArgs      string
	agentAddress   string
	agentMaxMem    uint64
	agentExtraArgs string
	agentUser      string
	cmd            *exec.Cmd
	pid            int
	snc            *Agent
	conf           *ConfigSection
	keepRunningA   atomic.Bool
	password       string
	urlPrefix      string
	listener       *Listener
	proxy          *httputil.ReverseProxy
	allowedHosts   *AllowedHostConfig
	initCallback   func()
}

// ensure we fully implement the RequestHandlerHTTP type
var _ RequestHandlerHTTP = &HandlerManagedExporter{}

// ExporterListenerExposed is a exporter which can be listed in the inventory
type ExporterListenerExposed interface {
	JSON() []map[string]string
}

// ensure we fully implement the ListExporter type
var _ RequestHandlerHTTP = &HandlerManagedExporter{}

// ensure managed exporters are listed in the inventory exports
var _ ExporterListenerExposed = &HandlerManagedExporter{}

func (l *HandlerManagedExporter) Type() string {
	return l.name
}

func (l *HandlerManagedExporter) BindString() string {
	return l.listener.BindString()
}

func (l *HandlerManagedExporter) Listener() *Listener {
	return l.listener
}

func (l *HandlerManagedExporter) Start() error {
	l.keepRunningA.Store(true)
	go func() {
		defer l.snc.logPanicExit()
		l.procMainLoop()
	}()

	return l.listener.Start()
}

func (l *HandlerManagedExporter) Stop() {
	l.keepRunningA.Store(false)
	l.listener.Stop()
	l.StopProc()
}

func (l *HandlerManagedExporter) StopProc() {
	if l.cmd != nil && l.cmd.Process != nil {
		l.cmd.WaitDelay = managedExporterRestartDelay
		LogDebug(l.cmd.Process.Kill())
		LogDebug(l.cmd.Wait())
	}
	l.cmd = nil
	l.pid = 0
}

func (l *HandlerManagedExporter) Init(snc *Agent, conf *ConfigSection, _ *Config, runSet *AgentRunSet) error {
	l.snc = snc
	l.conf = conf

	l.password = DefaultPassword
	if password, ok := conf.GetString("password"); ok {
		l.password = password
	}

	listener, err := SharedWebListener(snc, conf, l, runSet)
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
		return fmt.Errorf("agent path is required to start the %s agent", l.Type())
	}

	if agentArgs, ok := conf.GetString("agent args"); ok {
		l.agentArgs = agentArgs
	}

	if agentMaxMem, ok := conf.GetString("agent max memory"); ok {
		maxMem, err2 := humanize.ParseBytes(agentMaxMem)
		if err2 != nil {
			return fmt.Errorf("agent max memory: %s", err2.Error())
		}
		l.agentMaxMem = maxMem
	}

	if agentAddress, ok := conf.GetString("agent address"); ok {
		l.agentAddress = agentAddress
	}

	if agentUser, ok := conf.GetString("agent user"); ok {
		l.agentUser = agentUser
	}

	l.proxy = &httputil.ReverseProxy{
		Rewrite: func(proxyReq *httputil.ProxyRequest) {
			prefix := strings.TrimSuffix(l.urlPrefix, "/")
			proxyURL := "http://" + l.agentAddress + strings.TrimPrefix(proxyReq.In.URL.Path, prefix)
			if len(proxyReq.In.URL.Query()) > 0 {
				proxyURL = proxyURL + "?" + proxyReq.In.URL.Query().Encode()
			}
			uri, _ := url.Parse(proxyURL)
			proxyReq.Out.URL = uri
		},
		ErrorHandler: getReverseProxyErrorHandlerFunc(l.Type()),
	}

	allowedHosts, err := NewAllowedHostConfig(conf)
	if err != nil {
		return err
	}
	l.allowedHosts = allowedHosts

	if l.initCallback != nil {
		l.initCallback()
	}

	return nil
}

func (l *HandlerManagedExporter) GetAllowedHosts() *AllowedHostConfig {
	return l.allowedHosts
}

func (l *HandlerManagedExporter) CheckPassword(req *http.Request, _ URLMapping) bool {
	return verifyRequestPassword(l.snc, req, l.password)
}

func (l *HandlerManagedExporter) GetMappings(*Agent) []URLMapping {
	return []URLMapping{
		{URL: strings.TrimSuffix(l.urlPrefix, "/") + "/*", Handler: l.proxy},
	}
}

func (l *HandlerManagedExporter) JSON() []map[string]string {
	ssl := "0"
	if l.listener.tlsConfig != nil {
		ssl = "1"
	}

	return ([]map[string]string{{
		"bind": l.BindString(),
		"ssl":  ssl,
		"type": l.Type(),
		"name": l.name,
		"url":  l.GetMappings(nil)[0].URL,
	}})
}

func (l *HandlerManagedExporter) keepRunning() bool {
	return l.keepRunningA.Load()
}

func (l *HandlerManagedExporter) procMainLoop() {
	for l.keepRunning() {
		args := utils.Tokenize(l.agentArgs)
		if len(args) == 1 && args[0] == "" {
			args = []string{}
		}
		if l.agentExtraArgs != "" {
			extra := ReplaceMacros(l.agentExtraArgs, l.conf.data)
			args = append(args, extra)
		}
		cmd := exec.Command(l.agentPath, args...) //nolint:gosec // input source is the config file

		// drop privileges when started as root
		if l.agentUser != "" && os.Geteuid() == 0 {
			if err := setCmdUser(cmd, l.agentUser); err != nil {
				err = fmt.Errorf("failed to drop privileges for %s agent: %s", l.Type(), err.Error())
				log.Errorf("agent startup error: %s", err)

				return
			}
		}

		log.Debugf("starting %s agent: %s", l.Type(), cmd.Path)
		l.snc.passthroughLogs("stdout", "["+l.Type()+"] ", log.Debugf, cmd.StdoutPipe)
		l.snc.passthroughLogs("stderr", "["+l.Type()+"] ", l.logPass, cmd.StderrPipe)

		err := cmd.Start()
		if err != nil {
			err = fmt.Errorf("failed to start %s agent: %s", l.Type(), err.Error())
			log.Errorf("agent startup error: %s", err)

			return
		}

		l.pid = cmd.Process.Pid
		l.cmd = cmd

		if l.agentMaxMem > 0 {
			go func() {
				defer l.snc.logPanicExit()

				l.procMemWatcher()
			}()
		}

		err = cmd.Wait()
		if !l.keepRunning() {
			return
		}
		if err != nil {
			log.Errorf("%s agent errored: %s", l.Type(), err.Error())

			time.Sleep(managedExporterRestartDelay)
		}
	}
}

func (l *HandlerManagedExporter) procMemWatcher() {
	ticker := time.NewTicker(managedExporterMemWatchInterval)
	defer ticker.Stop()

	for {
		<-ticker.C
		if !l.keepRunning() {
			return
		}
		if l.cmd == nil {
			return
		}
		pid32, err := convert.Int32E(l.pid)
		if err != nil {
			log.Debugf("failed to convert pid %d: %s", l.pid, err.Error())

			return
		}

		proc, err := process.NewProcess(pid32)
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
			log.Warnf("%s memory usage - rss: %s (limit: %s), vms: %s -> restarting the agent process",
				l.name,
				humanize.BytesF(memInfo.RSS, 2),
				humanize.BytesF(l.agentMaxMem, 2),
				humanize.BytesF(memInfo.VMS, 2),
			)
			l.StopProc()
		} else {
			log.Tracef("%s memory usage - rss: %s (limit: %s), vms: %s",
				l.name,
				humanize.BytesF(memInfo.RSS, 2),
				humanize.BytesF(l.agentMaxMem, 2),
				humanize.BytesF(memInfo.VMS, 2),
			)
		}
	}
}

func (l *HandlerManagedExporter) logPass(f string, v ...interface{}) {
	entry := fmt.Sprintf(f, v...)
	switch {
	case strings.Contains(strings.ToLower(entry), "level=warn"):
		log.Warn(entry)
	case strings.Contains(strings.ToLower(entry), "level=info"):
		log.Debug(entry)
	case strings.Contains(strings.ToLower(entry), "level=debug"):
		log.Trace(entry)
	default:
		log.Error(entry)
	}
}
