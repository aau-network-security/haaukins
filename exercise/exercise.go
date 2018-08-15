package exercise

import (
	"errors"
	"fmt"

	"github.com/aau-network-security/go-ntp/virtual"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	)

var (
	DuplicateTagErr = errors.New("Tag already exists")
	MissingTagsErr  = errors.New("No tags, need atleast one tag")
	UnknownTagErr   = errors.New("Unknown tag")
)

type Flag struct {
}

type RecordConfig struct {
	Type string `yaml:"record"`
	Name string `yaml:"name"`
}

func (rc RecordConfig) Format(ip string) string {
	return fmt.Sprintf("%s %s %s", rc.Name, rc.Type, ip)
}

type FlagConfig struct {
	Name    string `yaml:"name"`
	EnvVar  string `yaml:"env"`
	Default string `yaml:"default"`
	Points  uint   `yaml:"points"`
}

type DockerConfig struct {
	Image    string         `yaml:"image"`
	Flags    []FlagConfig   `yaml:"flag"`
	Records  []RecordConfig `yaml:"dns"`
	MemoryMB uint           `yaml:"memoryMB"`
	CPU      float64        `yaml:"cpu"`
}

type Config struct {
	Name        string         `yaml:"name"`
	Tags        []string       `yaml:"tags"`
	DockerConfs []DockerConfig `yaml:"docker"`
	// VBoxConfig   []VBoxConfig   `yaml:"vbox"`
}

func (conf Config) Flags() []FlagConfig {
	var res []FlagConfig
	for _, dockerConf := range conf.DockerConfs {
		res = append(res, dockerConf.Flags...)
	}
	return res
}

func (ec Config) ContainerOpts() ([]docker.ContainerConfig, [][]RecordConfig) {
	var contSpecs []docker.ContainerConfig
	var contRecords [][]RecordConfig

	for _, conf := range ec.DockerConfs {
		spec := docker.ContainerConfig{
			Image: conf.Image,
			Resources: &docker.Resources{
				MemoryMB: conf.MemoryMB,
				CPU:      conf.CPU,
			},
		}

		contSpecs = append(contSpecs, spec)
		contRecords = append(contRecords, conf.Records)
	}

	return contSpecs, contRecords
}

type exercise struct {
	conf       *Config
	net        *docker.Network
	flags      []Flag
	machines   []virtual.Instance
	ips        []int
	dnsIP      string
	dnsRecords []string
}

func (e *exercise) Create() error {
	containers, records := e.conf.ContainerOpts()

	var machines []virtual.Instance
	for i, spec := range containers {
		spec.DNS = []string{e.dnsIP}

		c, err := docker.NewContainer(spec)
		if err != nil {
			return err
		}

		var lastDigit int
		// Example: 216

		if e.ips != nil {
			// Containers need specific ips
			lastDigit, err = e.net.Connect(c, e.ips[i])
			if err != nil {
				return err
			}
		} else {
			// Let network assign ips
			lastDigit, err = e.net.Connect(c)
			if err != nil {
				return err
			}

			e.ips = append(e.ips, lastDigit)
		}

		ipaddr := e.net.FormatIP(lastDigit)
		// Example: 172.16.5.216

		for _, record := range records[i] {
			e.dnsRecords = append(e.dnsRecords, record.Format(ipaddr))
		}

		machines = append(machines, c)
	}

	e.machines = machines

	return nil
}

func (e *exercise) Start() error {
	for _, m := range e.machines {
		if err := m.Start(); err != nil {
			return err
		}
	}
	return nil
}

func (e *exercise) Stop() error {
	for _, m := range e.machines {
		if err := m.Close(); err != nil {
			return err
		}
	}

	return nil
}

func (e *exercise) Close() error {
	for _, m := range e.machines {
		if err := m.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (e *exercise) Reset() error {
	if err := e.Stop(); err != nil {
		return err
	}

	if err := e.Start(); err != nil {
		return err
	}

	return nil
}
