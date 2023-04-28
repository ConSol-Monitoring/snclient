package snclient

type TaskSet struct {
	noCopy noCopy
	tasks  map[string]TaskHandler
}

func NewTaskSet() *TaskSet {
	ts := &TaskSet{
		tasks: make(map[string]TaskHandler),
	}

	return ts
}

func (ts *TaskSet) StopRemove() {
	for name, t := range ts.tasks {
		t.Stop()
		delete(ts.tasks, name)
	}
}

func (ts *TaskSet) Start() {
	for name := range ts.tasks {
		ts.StartTask(name)
	}
}

func (ts *TaskSet) Add(name string, task TaskHandler) {
	ts.tasks[name] = task
}

func (ts *TaskSet) StartTask(name string) {
	task, ok := ts.tasks[name]
	if !ok {
		log.Errorf("no task with name: %s", name)

		return
	}

	err := task.Start()
	if err != nil {
		log.Errorf("failed to start %s task: %s", name, err.Error())
		task.Stop()
		delete(ts.tasks, name)

		return
	}

	log.Tracef("task %s started", name)
}
