package snclient

func init() {
	AvailableTasks = append(AvailableTasks, NewTaskRunner("CheckSystemUnix", "/settings/system/unix", NewCheckSCheckSystemHandler))
}
