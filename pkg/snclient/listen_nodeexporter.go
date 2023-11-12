package snclient

func init() {
	RegisterModule(&AvailableListeners, "NodeExporterServer", "/settings/NodeExporter/server", NewHandlerNodeExporter)
}

func NewHandlerNodeExporter() Module {
	mod := &HandlerManagedExporter{
		name:           "nodeexporter",
		urlPrefix:      "/node",
		agentExtraArgs: "--web.listen-address=${agent address}",
		agentUser:      "nobody",
	}

	return mod
}
