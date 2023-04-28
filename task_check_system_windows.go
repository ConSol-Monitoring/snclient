package snclient

func init() {
	AvailableTasks = append(AvailableTasks, NewTaskRunner("CheckSystem", "/settings/system/windows", NewCheckSCheckSystemHandler))
}
