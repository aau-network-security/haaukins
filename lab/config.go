package lab

import (
	"io/ioutil"

	"github.com/aau-network-security/go-ntp/exercise"
	"gopkg.in/yaml.v2"
)

type capacity struct {
	Buffer int `yaml:"buffer"`
	Max    int `yaml:"max"`
}

type detail struct {
	HasNat  bool   `yaml:"has_nat"`
	OvaFile string `yaml:"ova_file"`
}

type frontends struct {
	Directory string   `yaml:"directory"`
	Details   []detail `yaml:"details"`
}

type Config struct {
	Capacity  capacity          `yaml:"capacity"`
	Frontends frontends         `yaml:"frontends"`
	Exercises []exercise.Config `yaml:"exercise"`
}

func (conf Config) Flags() []exercise.FlagConfig {
	var res []exercise.FlagConfig
	for _, exercise := range conf.Exercises {
		res = append(res, exercise.Flags()...)
	}
	return res
}

func LoadConfig(path string) (*Config, error) {
	var config *Config

	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(rawData, &config); err != nil {
		return nil, err
	}

	return config, nil
}
