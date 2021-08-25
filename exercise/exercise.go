// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package exercise

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"

	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/virtual"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/rs/zerolog/log"
)

var (
	DuplicateTagErr = errors.New("Tag already exists")
	MissingTagsErr  = errors.New("No tags, need atleast one tag")
	UnknownTagErr   = errors.New("Unknown tag")
	RegistryLink    = "registry.gitlab.com"
	tagRawRegexp    = `^[a-z0-9][a-z0-9-]*[a-z0-9]$`
	tagRegex        = regexp.MustCompile(tagRawRegexp)
	OvaSuffix       = ".ova"
)

type DockerHost interface {
	CreateContainer(ctx context.Context, conf docker.ContainerConfig) (docker.Container, error)
}

type dockerHost struct{}

func (dockerHost) CreateContainer(ctx context.Context, conf docker.ContainerConfig) (docker.Container, error) {
	c := docker.NewContainer(conf)
	err := c.Create(ctx)

	return c, err
}

type exercise struct {
	containerOpts []store.ContainerOptions
	vboxOpts      []store.ExerciseInstanceConfig

	tag store.Tag

	dhost DockerHost
	vlib  vbox.Library
	net   docker.Network

	dnsAddr    string
	dnsRecords []store.RecordConfig

	ips      []int
	machines []virtual.Instance
}

func NewExercise(conf store.Exercise, dhost DockerHost, vlib vbox.Library, net docker.Network, dnsAddr string) *exercise {
	var containerOpts []store.ContainerOptions
	var vboxOpts []store.ExerciseInstanceConfig
	var ex *exercise
	for _, c := range conf.Instance {
		if strings.Contains(c.Image, OvaSuffix) {
			vboxOpts = append(vboxOpts, c)
		} else {
			containerOpts = conf.ContainerOpts()
			break
		}
	}

	if !conf.Static {
		ex = &exercise{
			containerOpts: containerOpts,
			vboxOpts:      vboxOpts,
			tag:           conf.Tag,
			dhost:         dhost,
			vlib:          vlib,
			net:           net,
			dnsAddr:       dnsAddr,
		}
	} else {
		ex = &exercise{
			containerOpts: containerOpts,
			tag:           conf.Tag,
			dhost:         dhost,
		}
	}
	return ex

}

func (e *exercise) Create(ctx context.Context) error {
	var machines []virtual.Instance
	var newIps []int
	for i, opt := range e.containerOpts {
		opt.DockerConf.DNS = []string{e.dnsAddr}
		opt.DockerConf.Labels = map[string]string{
			"hkn": "lab_exercise",
		}

		c, err := e.dhost.CreateContainer(ctx, opt.DockerConf)
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
		vmConf := store.InstanceConfig{
			Image:    vboxConf.Image,
			CPU:      vboxConf.CPU,
			MemoryMB: vboxConf.MemoryMB,
		}
		vm, err := e.vlib.GetCopy(
			ctx,
			vmConf,
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

func (e *exercise) Start(ctx context.Context) error {
	var res error
	var wg sync.WaitGroup

	for _, m := range e.machines {
		wg.Add(1)
		go func(m virtual.Instance) {
			if m.Info().State != virtual.Running {
				if err := m.Start(ctx); err != nil && res == nil {
					res = err
				}
			}
			wg.Done()
		}(m)
	}
	wg.Wait()

	return res
}

func (e *exercise) Suspend(ctx context.Context) error {
	for _, m := range e.machines {
		if m.Info().State == virtual.Running {
			if err := m.Suspend(ctx); err != nil {
				return err
			}
		}
	}

	return nil
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

func (e *exercise) Reset(ctx context.Context) error {
	if err := e.Close(); err != nil {
		return err
	}

	if err := e.Create(ctx); err != nil {
		return err
	}

	if err := e.Start(ctx); err != nil {
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
				Name:  f.Name,
				Tag:   f.Tag,
				Value: f.StaticFlag,
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
