package daemon

import (
	"io/ioutil"
	"sync"

	yaml "gopkg.in/yaml.v2"

	"github.com/aau-network-security/go-ntp/virtual/docker"
	dockerclient "github.com/fsouza/go-dockerclient"
)

type Config struct {
	Path string `yaml:"-"`
	m    sync.Mutex

	Host               string                           `yaml:"host"`
	SecretSigningKey   string                           `yaml:"signing-key"`
	OvaDir             string                           `yaml:"ova-directory"`
	Users              []User                           `yaml:"users,omitempty"`
	SignupKeys         []SignupKey                      `yaml:"signup-keys,omitempty"`
	DockerRepositories []dockerclient.AuthConfiguration `yaml:"docker-repos,omitempty"`
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

	c.Path = path

	return &c, nil
}

func (c *Config) AddUser(u *User) error {
	c.m.Lock()
	defer c.m.Unlock()

	c.Users = append(c.Users, *u)

	return c.Save()
}

func (c *Config) DeleteUserByUsername(username string) error {
	c.m.Lock()
	defer c.m.Unlock()

	for i, cu := range c.Users {
		if username == cu.Username {
			c.Users = append(c.Users[:i], c.Users[i+1:]...)
			return c.Save()
		}
	}

	return UnknownUserErr
}

func (c *Config) AddSignupKey(k SignupKey) error {
	c.m.Lock()
	defer c.m.Unlock()

	c.SignupKeys = append(c.SignupKeys, k)

	return c.Save()
}

func (c *Config) DeleteSignupKey(k SignupKey) error {
	c.m.Lock()
	defer c.m.Unlock()

	for i, ck := range c.SignupKeys {
		if k == ck {
			c.SignupKeys = append(c.SignupKeys[:i], c.SignupKeys[i+1:]...)
			return c.Save()
		}
	}

	return UnknownSignupKey
}

func (c *Config) Save() error {
	bytes, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(c.Path, bytes, 0644); err != nil {
		return err
	}

	return nil
}
