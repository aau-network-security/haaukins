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

type FlagConfig struct {
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

func (ec Config) ContainerOpts() ([]docker.ContainerConfig, [][]string) {
	var contSpecs []docker.ContainerConfig
	var contRecords [][]string

	for _, conf := range ec.DockerConfs {
		spec := docker.ContainerConfig{
			Image: conf.Image,
			Resources: &docker.Resources{
				MemoryMB: conf.MemoryMB,
				CPU:      conf.CPU,
			},
		}

		var records []string
		for _, r := range conf.Records {
			records = append(records, fmt.Sprintf("%s %s", r.Name, r.Type))
		}

		contSpecs = append(contSpecs, spec)
		contRecords = append(contRecords, records)
	}

	return contSpecs, contRecords
}

type exercise struct {
	conf       *Config
	net        *docker.Network
	flags      []Flag
	machines   []virtual.Instance
	dnsIP      string
	dnsRecords []string
}

func (e *exercise) Start() error {
	containers, records := e.conf.ContainerOpts()

	var machines []virtual.Instance
	for i, spec := range containers {
		spec.DNS = []string{e.dnsIP}

		c, err := docker.NewContainer(spec)
		if err != nil {
			return err
		}

		if err := c.Start(); err != nil {
			return err
		}

		lastDigit, err := e.net.Connect(c, lastDigit...)
		if err != nil {
			return err
		}

		ipaddr := e.net.FormatIP(lastDigit)

		var finalRecords []string
		for _, record := range records[i] {
			finalRecords = append(finalRecords, fmt.Sprintf("%s %s", record, ipaddr))
		}
		e.dnsRecords = finalRecords

		machines = append(machines, c)
	}

	e.machines = machines

	return nil
}

func (e *exercise) Stop() error {
	for _, m := range e.machines {
		if err := m.Kill(); err != nil {
			return err
		}
	}

	return nil
}

func (e *exercise) Reset() error {
	if err := e.Stop(); err != nil {
		return err
	}
	// NOT FUNCTIONING: Need to handle that docker containers are spun up on the same IP
	if err := e.Start(); err != nil {
		return err
	}

	return nil
}
