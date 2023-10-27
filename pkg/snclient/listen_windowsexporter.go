package snclient

func init() {
	RegisterModule(&AvailableListeners, "WindowsExporterServer", "/settings/WindowsExporter/server", NewHandlerWindowsExporter)
}

func NewHandlerWindowsExporter() Module {
	mod := &HandlerManagedExporter{
		name:           "windowsexporter",
		urlPrefix:      "/node",
		agentExtraArgs: "--web.listen-address=${agent address}",
	}

	return mod
}
