package ntp

import (
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type dnsRecordConfig struct {
	Type string `yaml:"record"`
	Name string `yaml:"name"`
}

type flagConfig struct {
	EnvVar  string `yaml:"env"`
	Default string `yaml:"default"`
	Points  uint   `yaml:"points"`
}

type dockerConfig struct {
	Image    string            `yaml:"image"`
	Flags    []flagConfig      `yaml:"flag"`
	Records  []dnsRecordConfig `yaml:"dns"`
	MemoryMB uint              `yaml:"memoryMB"`
	CPU      float64           `yaml:"cpu"`
}

type vBoxConfig struct {
	OvaFile     string  `yaml:"image"`
	MemoryMB    uint    `yaml:"memoryMB"`
	CPU         float64 `yaml:"cpu"`
	RDP         bool    `yaml:"rdp"`
	PromiscMode bool    `yaml:"promisc"`
}

type exerciseConfig struct {
	Name        string         `yaml:"name"`
	Tags        []string       `yaml:"tags"`
	DockerConfs []dockerConfig `yaml:"docker"`
}

type Config interface {
	ByTag(string) *exerciseConfig
}

type config struct {
	Exercises   []*exerciseConfig `yaml:"exercise"`
	tagExercise map[string]*exerciseConfig
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
