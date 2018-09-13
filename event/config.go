package event

import (
	"io/ioutil"

	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
	"github.com/aau-network-security/go-ntp/svcs/revproxy"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Name  string           `yaml:"name"`
	Tag   string           `yaml:"tag"`
	CTFd  ctfd.Config      `yaml:"ctfd"`
	Guac  guacamole.Config `yaml:"guacamole"`
	Proxy revproxy.Config  `yaml:"revproxy"`
	Lab   lab.Config       `yaml:"lab"`
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
