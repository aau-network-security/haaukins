package event

import (
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
	"github.com/aau-network-security/go-ntp/svcs/revproxy"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type Config struct {
	ctfd     ctfd.Config      `yaml:"ctfd"`
	guac     guacamole.Config `yaml:"guacamole"`
	revproxy revproxy.Config  `yaml:"revproxy"`
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
