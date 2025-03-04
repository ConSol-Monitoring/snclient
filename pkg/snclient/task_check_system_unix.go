//go:build !windows

package snclient

func init() {
	RegisterModule(
		&AvailableTasks,
		"CheckSystemUnix",
		"/settings/system/unix",
		NewCheckSystemHandler,
		ConfigInit{
			DefaultSystemTaskConfig,
		},
	)
}
