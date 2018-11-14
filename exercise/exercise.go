package exercise

import (
	"errors"
	"regexp"

	"sync"

	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/virtual"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/rs/zerolog/log"
)

var (
	DuplicateTagErr = errors.New("Tag already exists")
	MissingTagsErr  = errors.New("No tags, need atleast one tag")
	UnknownTagErr   = errors.New("Unknown tag")

	tagRawRegexp = `^[a-z0-9][a-z0-9-]*[a-z0-9]$`
	tagRegex     = regexp.MustCompile(tagRawRegexp)
)

type DockerHost interface {
	CreateContainer(conf docker.ContainerConfig) (docker.Container, error)
}

type dockerHost struct{}

func (dockerHost) CreateContainer(conf docker.ContainerConfig) (docker.Container, error) {
	return docker.NewContainer(conf)
}

type exercise struct {
	containerOpts []store.ContainerOptions
	vboxOpts      []store.ExerciseInstanceConfig

	dhost DockerHost
	vlib  vbox.Library
	net   docker.Network

	dnsAddr    string
	dnsRecords []store.RecordConfig

	ips      []int
	machines []virtual.Instance
}

func NewExercise(conf store.Exercise, dhost DockerHost, vlib vbox.Library, net docker.Network, dnsAddr string) *exercise {
	containerOpts := conf.ContainerOpts()

	var vboxOpts []store.ExerciseInstanceConfig
	for _, vboxConf := range conf.VboxConfs {
		vboxOpts = append(vboxOpts, vboxConf.ExerciseInstanceConfig)
	}

	return &exercise{
		containerOpts: containerOpts,
		vboxOpts:      vboxOpts,

		dhost:   dhost,
		vlib:    vlib,
		net:     net,
		dnsAddr: dnsAddr,
	}
}

func (e *exercise) Create() error {
	var machines []virtual.Instance
	var newIps []int
	for i, opt := range e.containerOpts {
		opt.DockerConf.DNS = []string{e.dnsAddr}

		c, err := e.dhost.CreateContainer(opt.DockerConf)
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

			newIps = append(newIps, lastDigit)
		}

		ipaddr := e.net.FormatIP(lastDigit)
		// Example: 172.16.5.216

		for _, record := range opt.Records {
			if record.RData == "" {
				record.RData = ipaddr
			}
			e.dnsRecords = append(e.dnsRecords, record)
		}

		machines = append(machines, c)
	}

	for _, vboxConf := range e.vboxOpts {
		vm, err := e.vlib.GetCopy(
			vboxConf.InstanceConfig,
			vbox.SetBridge(e.net.Interface()),
		)
		if err != nil {
			return err
		}
		machines = append(machines, vm)
	}

	if e.ips == nil {
		e.ips = newIps
	}

	e.machines = machines

	return nil
}

func (e *exercise) Start() error {
	var res error
	var wg sync.WaitGroup
	for _, m := range e.machines {
		wg.Add(1)
		go func(m virtual.Instance) {
			if err := m.Start(); err != nil && res == nil {
				res = err
			}
		}(m)
		wg.Done()
	}
	wg.Wait()
	return res
}

func (e *exercise) Stop() error {
	for _, m := range e.machines {
		if err := m.Stop(); err != nil {
			return err
		}
	}

	return nil
}

func (e *exercise) Close() error {
	var wg sync.WaitGroup

	for _, m := range e.machines {
		wg.Add(1)
		go func(i virtual.Instance) {
			if err := i.Close(); err != nil {
				log.Warn().Msgf("error while closing exercise: %s", err)
			}
			wg.Done()
		}(m)

	}
	wg.Wait()

	e.machines = nil
	return nil
}

func (e *exercise) Restart() error {
	if err := e.Stop(); err != nil {
		return err
	}

	if err := e.Start(); err != nil {
		return err
	}

	return nil
}

func (e *exercise) Reset() error {
	if err := e.Close(); err != nil {
		return err
	}

	if err := e.Create(); err != nil {
		return err
	}

	if err := e.Start(); err != nil {
		return err
	}

	return nil
}

func (e *exercise) Challenges() []store.Challenge {
	var challenges []store.Challenge

	for _, opt := range e.containerOpts {
		challenges = append(challenges, opt.Challenges...)
	}

	for _, opt := range e.vboxOpts {
		for _, f := range opt.Flags {
			challenges = append(challenges, store.Challenge{
				FlagTag:   f.Tag,
				FlagValue: f.Static,
			})
		}
	}

	return challenges
}

func (e *exercise) InstanceInfo() []virtual.InstanceInfo {
	var instances []virtual.InstanceInfo
	for _, m := range e.machines {
		instances = append(instances, m.Info())
	}
	return instances
}