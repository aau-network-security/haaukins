package ntp

import (
	"fmt"
	"io/ioutil"

	"github.com/aau-network-security/go-ntp/virtual"

	yaml "gopkg.in/yaml.v2"
)

type Stopper interface {
	Stop() error
}

type Starter interface {
	Start() error
}

type StartStopper interface {
	Starter
	Stopper
}

type MachineConfig struct {
	Image    virtual.ID     `yaml:"image"`
	Flags    []FlagConfig   `yaml:"flag"`
	Records  []RecordConfig `yaml:"dns"`
	MemoryMB uint           `yaml:"memoryMB"`
	CPU      float64        `yaml:"cpu"`
}

func Restart(ss StartStopper) error {
	if err := ss.Stop(); err != nil {
		return err
	}

	if err := ss.Start(); err != nil {
		return err
	}

	return nil
}

type CheckPointer interface {
	Points() []CheckPoint
}

type CheckPoint struct {
	Name  string
	Value string
	Score uint
}

type vBoxConfig struct {
	OvaFile     string  `yaml:"image"`
	MemoryMB    uint    `yaml:"memoryMB"`
	CPU         float64 `yaml:"cpu"`
	RDP         bool    `yaml:"rdp"`
	PromiscMode bool    `yaml:"promisc"`
}

type Config interface {
	ByTag(string) *exerciseConfig
}

type config struct {
	Exercises   []*ExerciseConfig `yaml:"exercise"`
	tagExercise map[string]*ExerciseConfig
}

func (c *config) ByTag(t string) *exerciseConfig {
	return c.tagExercise[t]
}

func LoadConfig(path string) (Config, error) {
	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var conf config
	if err := yaml.Unmarshal(rawData, &conf); err != nil {
		return nil, err
	}

	conf.tagExercise = make(map[string]*exerciseConfig)
	for _, e := range conf.Exercises {
		for _, t := range e.Tags {
			exer, ok := conf.tagExercise[t]
			if ok {
				return nil, fmt.Errorf("Redundant tag \"%s\" (used for: %s and %s)", t, exer.Name, e.Name)
			}
			conf.tagExercise[t] = e
		}
	}

	return &conf, nil
}
