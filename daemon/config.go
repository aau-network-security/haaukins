package daemon

import (
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"

	"github.com/aau-network-security/go-ntp/virtual/docker"
	dockerclient "github.com/fsouza/go-dockerclient"
)

type Config struct {
	Host               string                           `yaml:"host"`
	SecretSigningKey   string                           `yaml:"signing-key"`
	OvaDir             string                           `yaml:"ova-directory"`
	DockerRepositories []dockerclient.AuthConfiguration `yaml:"docker-repositories,omitempty"`
	TLS                struct {
		Management struct {
			CertFile string `yaml:"cert-file"`
			KeyFile  string `yaml:"key-file"`
		} `yaml:"management"`
	} `yaml:"tls,omitempty"`
}

func NewConfigFromFile(path string) (*Config, error) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var c Config
	err = yaml.Unmarshal(f, &c)
	if err != nil {
		return nil, err
	}

	for _, repo := range c.DockerRepositories {
		docker.Registries[repo.ServerAddress] = repo
	}

	return &c, nil
}
