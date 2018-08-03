package lab

import (
	"io/ioutil"

	"github.com/aau-network-security/go-ntp/exercise"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Exercises []exercise.Config `yaml:"exercise"`
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
