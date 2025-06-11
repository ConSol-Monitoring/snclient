package snclient

import (
	"fmt"
)

var moduleConfigDefaults map[string]ConfigInit

// Module is a generic module interface to abstract optional agent functionality
type Module interface {
	Init(snc *Agent, section *ConfigSection, cfg *Config, runSet *AgentRunSet) error
	Start() error
	Stop()
}

// ModuleSet is a list of modules sharing a common type
type ModuleSet struct {
	noCopy  noCopy
	name    string
	modules map[string]Module
}

func NewModuleSet(name string) *ModuleSet {
	ms := &ModuleSet{
		name:    name,
		modules: make(map[string]Module),
	}

	return ms
}

func (ms *ModuleSet) Stop() {
	for _, t := range ms.modules {
		t.Stop()
	}
}

func (ms *ModuleSet) StopRemove() {
	for name, t := range ms.modules {
		t.Stop()
		delete(ms.modules, name)
	}
}

func (ms *ModuleSet) Start() {
	for name := range ms.modules {
		ms.startModule(name)
	}
}

func (ms *ModuleSet) Get(name string) (task Module) {
	if task, ok := ms.modules[name]; ok {
		return task
	}

	return nil
}

func (ms *ModuleSet) Add(name string, task Module) error {
	if _, ok := ms.modules[name]; ok {
		if mod, ok := task.(RequestHandler); ok {
			name = name + ":" + mod.Type()
		}
		if _, ok := ms.modules[name]; ok {
			return fmt.Errorf("duplicate %s module with name: %s", ms.name, name)
		}
	}
	ms.modules[name] = task

	return nil
}

func (ms *ModuleSet) startModule(name string) {
	module, ok := ms.modules[name]
	if !ok {
		log.Errorf("no %s module with name: %s", ms.name, name)

		return
	}

	err := module.Start()
	if err != nil {
		log.Errorf("failed to start %s %s module: %s", name, ms.name, err.Error())
		module.Stop()
		delete(ms.modules, name)

		return
	}

	log.Tracef("module %s started", name)
}

// LoadableModule is a module which can be enabled by config
type LoadableModule struct {
	noCopy    noCopy
	ModuleKey string
	ConfigKey string
	Creator   func() Module
	Handler   *Module
}

// RegisterModule creates a new Module object and puts it on the list of available modules.
func RegisterModule(list *[]*LoadableModule, moduleKey, confKey string, creator func() Module, confInit ConfigInit) *LoadableModule {
	module := LoadableModule{
		ModuleKey: moduleKey,
		ConfigKey: confKey,
		Creator:   creator,
	}
	*list = append(*list, &module)

	if moduleConfigDefaults == nil {
		moduleConfigDefaults = map[string]ConfigInit{}
	}

	if _, ok := moduleConfigDefaults[confKey]; ok {
		log.Panicf("module section %s registered twice", confKey)
	}
	moduleConfigDefaults[confKey] = confInit

	return &module
}

// Name returns name of this module
func (lm *LoadableModule) Name() string {
	return lm.ModuleKey
}

// Init creates the actual TaskHandler for this task
func (lm *LoadableModule) Init(snc *Agent, conf *Config, runSet *AgentRunSet) (Module, error) {
	handler := lm.Creator()
	modConf := conf.Section(lm.ConfigKey)

	err := handler.Init(snc, modConf, conf, runSet)
	if err != nil {
		return nil, fmt.Errorf("%s init failed: %s", lm.Name(), err.Error())
	}

	return handler, nil
}
