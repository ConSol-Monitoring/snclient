package snclient

func init() {
	RegisterModule(&AvailableListeners, "WindowsExporterServer", "/settings/WindowsExporter/server", NewHandlerWindowsExporter)
}

func NewHandlerWindowsExporter() Module {
	l := &HandlerNodeExporter{
		name: "windowsexporter",
	}

	return l
}
