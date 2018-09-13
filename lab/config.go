package lab

import (
	"io/ioutil"

	"github.com/aau-network-security/go-ntp/exercise"
	"gopkg.in/yaml.v2"
)

type LabConfig struct {
	Frontends []string          `yaml:"frontends"`
	Exercises []exercise.Config `yaml:"exercise"`
}

func (conf LabConfig) Flags() []exercise.FlagConfig {
	var res []exercise.FlagConfig
	for _, exercise := range conf.Exercises {
		res = append(res, exercise.Flags()...)
	}
	return res
}

func LoadConfig(path string) (*LabConfig, error) {
	var config *LabConfig

	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(rawData, &config); err != nil {
		return nil, err
	}

	return config, nil
}
