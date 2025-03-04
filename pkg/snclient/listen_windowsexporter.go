package snclient

import (
	"os"
	"path/filepath"
)

func init() {
	RegisterModule(
		&AvailableListeners,
		"WindowsExporterServer",
		"/settings/WindowsExporter/server",
		NewHandlerWindowsExporter,
		ConfigInit{
			ConfigData{
				"agent path":       "${shared-path}/exporter/windows_exporter.exe",
				"agent args":       "",
				"agent address":    "127.0.0.1:9990",
				"agent max memory": "256M",
				"port":             "${/settings/WEB/server/port}",
				"use ssl":          "${/settings/WEB/server/use ssl}",
				"url prefix":       "/node",
			},
			"/settings/default",
			DefaultListenHTTPConfig,
		},
	)
}

func NewHandlerWindowsExporter() Module {
	mod := &HandlerManagedExporter{
		name:           "windowsexporter",
		agentExtraArgs: "--web.listen-address=${agent address}",
	}

	mod.initCallback = func() {
		// create textfile_inputs folder, otherwise the windows export complains about that every second
		base, _ := mod.conf.GetString("agent path")
		if base == "" {
			return
		}
		base = filepath.Dir(base)
		folder := filepath.Join(base, "textfile_inputs")
		if _, err := os.ReadDir(folder); err == nil {
			return
		}

		err := os.Mkdir(folder, 0o700)
		if err != nil {
			log.Debugf("mkdir %s: %s", folder, err.Error())
		}
	}

	return mod
}
