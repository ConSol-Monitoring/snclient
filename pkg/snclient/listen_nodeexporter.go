package snclient

func init() {
	RegisterModule(
		&AvailableListeners,
		"NodeExporterServer",
		"/settings/NodeExporter/server",
		NewHandlerNodeExporter,
		ConfigInit{
			ConfigData{
				"agent path":       "/usr/lib/snclient/node_exporter",
				"agent args":       "",
				"agent address":    "127.0.0.1:9990",
				"agent max memory": "256M",
				"agent user":       "nobody",
				"port":             "${/settings/WEB/server/port}",
				"use ssl":          "${/settings/WEB/server/use ssl}",
				"url prefix":       "/node",
			},
			"/settings/default",
			DefaultListenHTTPConfig,
		})
}

func NewHandlerNodeExporter() Module {
	mod := &HandlerManagedExporter{
		name:           "nodeexporter",
		agentExtraArgs: "--web.listen-address=${agent address}",
	}

	return mod
}
