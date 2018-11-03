package store

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"sync"
)

type FrontendStore interface {
	GetFrontends(...string) []InstanceConfig
	SetMemoryMB(string, uint)
	SetCpu(string, float64)
	runHooks() error
}

func NewFrontendsFile(path string) (FrontendStore, error) {
	var conf struct {
		Frontends []InstanceConfig `yaml:"frontends"`
	}

	var m sync.Mutex
	save := func() error {
		m.Lock()
		defer m.Unlock()

		bytes, err := yaml.Marshal(conf)
		if err != nil {
			return err
		}

		return ioutil.WriteFile(path, bytes, 0644)
	}

	// file exists
	var frontends map[string]InstanceConfig
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		f, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(f, &conf)
		if err != nil {
			return nil, err
		}

		for _, f := range conf.Frontends {
			if err := f.Validate(); err != nil {
				return nil, err
			}
			frontends[f.Image] = f
		}
	}

	return &frontendstore{
		frontends: frontends,
		hooks: []func([]InstanceConfig) error{
			func(ics []InstanceConfig) error {
				conf.Frontends = ics
				return save()
			},
		},
	}, nil
}

type frontendstore struct {
	frontends map[string]InstanceConfig
	hooks     []func([]InstanceConfig) error
}

func (fs *frontendstore) GetFrontends(names ...string) []InstanceConfig {
	var res []InstanceConfig
	for _, name := range names {
		ic, ok := fs.frontends[name]
		if !ok {
			ic = InstanceConfig{Image: name}
		}
		res = append(res, ic)
	}
	return res
}

func (fs *frontendstore) SetMemoryMB(f string, memoryMB uint) {
	ic, ok := fs.frontends[f]
	if !ok {
		ic = InstanceConfig{
			Image: f,
		}
	}
	ic.MemoryMB = memoryMB
	fs.frontends[f] = ic

	fs.runHooks()
}

func (fs *frontendstore) SetCpu(f string, cpu float64) {
	ic, ok := fs.frontends[f]
	if !ok {
		ic = InstanceConfig{
			Image: f,
		}
	}
	ic.CPU = cpu
	fs.frontends[f] = ic

	fs.runHooks()
}

func (fs *frontendstore) runHooks() error {
	var frontends []InstanceConfig
	for _, f := range fs.frontends {
		frontends = append(frontends, f)
	}

	for _, h := range fs.hooks {
		if err := h(frontends); err != nil {
			return err
		}
	}
	return nil
}
