package event

import (
	"io/ioutil"

	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"gopkg.in/yaml.v2"
)

// type Config struct {
// 	Name string        `yaml:"name"`
// 	Tag  string        `yaml:"tag"`
// 	Lab  lab.LabConfig `yaml:"lab"`

// 	// CTFd  ctfd.Config      `yaml:"ctfd"`
// 	// Guac  guacamole.Config `yaml:"guacamole"`
// 	// Proxy revproxy.Config  `yaml:"revproxy"`
// }

type Config struct {
	Name        string `json:"name"`
	Tag         string `json:"tag"`
	Buffer      int    `json:"buffer"`
	Capacity    int    `json:"capacity"`
	LabConfig   lab.LabConfig
	VBoxLibrary vbox.Library
}

func loadConfig(path string) (*Config, error) {
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
