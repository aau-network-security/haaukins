// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package lab

import (
	"context"
	"math/rand"
	"time"

	"io"
	"sync"

	"github.com/aau-network-security/haaukins/exercise"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/virtual"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/rs/zerolog/log"
)

var (
	newEnvironment = exercise.NewEnvironment
)

type Config struct {
	Frontends []store.InstanceConfig
	Exercises []store.Exercise
}

func (conf Config) Flags() []store.FlagConfig {
	var res []store.FlagConfig
	for _, exercise := range conf.Exercises {
		res = append(res, exercise.Flags()...)
	}
	return res
}

type LabHost interface {
	NewLab(context.Context, vbox.Library, Config) (Lab, error)
}

type labHost struct{}

func (lh *labHost) NewLab(ctx context.Context, lib vbox.Library, config Config) (Lab, error) {
	env := newEnvironment(lib)
	if err := env.Create(ctx); err != nil {
		return nil, err
	}

	if err := env.Add(ctx, config.Exercises...); err != nil {
		return nil, err
	}

	dockerHost := docker.NewHost()
	l := &lab{
		tag:         generateTag(),
		lib:         lib,
		environment: env,
		dockerHost:  dockerHost,
		frontends:   map[uint]frontendConf{},
	}

	for _, f := range config.Frontends {
		port := virtual.GetAvailablePort()
		if _, err := l.addFrontend(ctx, f, port); err != nil {
			return nil, err
		}
	}

	return l, nil
}

type Lab interface {
	Start(context.Context) error
	Stop() error
	Restart(context.Context) error
	GetEnvironment() exercise.Environment
	ResetFrontends(ctx context.Context) error
	RdpConnPorts() []uint
	GetTag() string
	InstanceInfo() []virtual.InstanceInfo
	io.Closer
}

type lab struct {
	tag         string
	lib         vbox.Library
	environment exercise.Environment
	frontends   map[uint]frontendConf
	dockerHost  docker.Host
}

type frontendConf struct {
	vm   vbox.VM
	conf store.InstanceConfig
}

func (l *lab) addFrontend(ctx context.Context, conf store.InstanceConfig, rdpPort uint) (vbox.VM, error) {
	hostIp, err := l.dockerHost.GetDockerHostIP()
	if err != nil {
		return nil, err
	}

	vm, err := l.lib.GetCopy(ctx,
		conf,
		vbox.SetBridge(l.environment.NetworkInterface()),
		vbox.SetLocalRDP(hostIp, rdpPort),
		vbox.SetRAM(conf.MemoryMB),
	)
	if err != nil {
		return nil, err
	}

	l.frontends[rdpPort] = frontendConf{
		vm:   vm,
		conf: conf,
	}

	log.Debug().Msgf("Created lab frontend on port %d", rdpPort)

	return vm, nil
}

func (l *lab) GetEnvironment() exercise.Environment {
	return l.environment
}

func (l *lab) ResetFrontends(ctx context.Context) error {
	var errs []error
	for p, vmConf := range l.frontends {
		err := vmConf.vm.Close()
		if err != nil {
			errs = append(errs, err)
			continue
		}

		vm, err := l.addFrontend(ctx, vmConf.conf, p)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		err = vm.Start(ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}
	}

	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

func (l *lab) Start(ctx context.Context) error {
	if err := l.environment.Start(ctx); err != nil {
		return err
	}

	for _, fconf := range l.frontends {
		if err := fconf.vm.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (l *lab) Stop() error {
	if err := l.environment.Stop(); err != nil {
		return err
	}

	for _, fconf := range l.frontends {
		if err := fconf.vm.Stop(); err != nil {
			return err
		}
	}

	return nil
}

func (l *lab) Restart(ctx context.Context) error {
	if err := l.environment.Stop(); err != nil {
		return err
	}

	if err := l.environment.Start(ctx); err != nil {
		return err
	}

	for _, fconf := range l.frontends {
		if err := fconf.vm.Stop(); err != nil {
			return err
		}

		if err := fconf.vm.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (l *lab) Close() error {
	var wg sync.WaitGroup

	closer := func(c io.Closer) {
		if err := c.Close(); err != nil {
			log.Warn().Msgf("error while closing lab: %s", err)
		}
		wg.Done()
	}

	if l.environment != nil {
		wg.Add(1)
		go closer(l.environment)
	}

	wg.Wait()

	return nil
}

func (l *lab) RdpConnPorts() []uint {
	var ports []uint
	for p, _ := range l.frontends {
		ports = append(ports, p)
	}

	return ports
}

func (l *lab) GetTag() string {
	return l.tag
}

func (l *lab) InstanceInfo() []virtual.InstanceInfo {
	var instances []virtual.InstanceInfo
	for _, fconf := range l.frontends {
		instances = append(instances, fconf.vm.Info())
	}
	instances = append(instances, l.environment.InstanceInfo()...)
	return instances
}

func generateTag() string {
	// seed for our GetRandomName
	rand.Seed(time.Now().UnixNano())
	return namesgenerator.GetRandomName(0)
}
