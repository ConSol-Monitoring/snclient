package snclient

import "fmt"

type TaskHandler interface {
	Defaults() ConfigData
	Init(*Agent, *ConfigSection, *Config) error
	Start() error
	Stop()
}

var AvailableTasks []*TaskRunner

// TaskRunner is a generic runner to do background jobs
type TaskRunner struct {
	noCopy    noCopy
	ModuleKey string
	ConfigKey string
	Creator   func() TaskHandler
	Handler   *TaskHandler
}

// NewTaskRunner creates a new TaskRunner object.
func NewTaskRunner(moduleKey, confKey string, creator func() TaskHandler) *TaskRunner {
	task := TaskRunner{
		ModuleKey: moduleKey,
		ConfigKey: confKey,
		Creator:   creator,
	}

	return &task
}

// Name returns name of this runner
func (tr *TaskRunner) Name() string {
	return tr.ModuleKey
}

// Init creates the actual TaskHandler for this task
func (tr *TaskRunner) Init(snc *Agent, conf *Config) (TaskHandler, error) {
	task := tr.Creator()

	taskConf := conf.Section(tr.ConfigKey).Clone()
	taskConf.data.Merge(conf.Section("/settings/default").data)
	taskConf.data.Merge(task.Defaults())

	err := task.Init(snc, taskConf, conf)
	if err != nil {
		task.Stop()

		return nil, fmt.Errorf("%s init failed: %s", tr.Name(), err.Error())
	}

	return task, nil
}
