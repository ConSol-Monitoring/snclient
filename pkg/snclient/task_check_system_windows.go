package snclient

func init() {
	RegisterModule(&AvailableTasks, "CheckSystem", "/settings/system/windows", NewCheckSystemHandler)
}
