// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package lab

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

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
	Frontends         []store.InstanceConfig
	Exercises         []store.Exercise
	DisabledExercises []store.Tag
}

func (conf Config) Flags() []store.ChildrenChalConfig {
	var res []store.ChildrenChalConfig
	for _, exercise := range conf.Exercises {
		res = append(res, exercise.Flags()...)
	}
	return res
}

// GetChildrenChallenges returns list of children challenge tags to be used in amigo frontend
func (conf Config) GetChildrenChallenges(parentTag string) []string {
	var childrenTags []string
	var flags []store.ChildrenChalConfig
	for _, i := range conf.Exercises {
		if i.Tag == store.Tag(parentTag) {
			for _, m := range i.Instance {
				flags = append(flags, m.Flags...)
			}
		}
	}
	for _, f := range flags {
		childrenTags = append(childrenTags, string(f.Tag))
	}
	return childrenTags
}

type Creator interface {
	NewLab(context.Context, int32) (Lab, error)
	UpdateExercises([]store.Exercise)
}

type LabHost struct {
	Vlib vbox.Library
	Conf Config
}

func (lh *LabHost) UpdateExercises(newExercises []store.Exercise) {
	newExercises = append(newExercises, lh.Conf.Exercises...)
	lh.Conf.Exercises = newExercises
}

func (lh *LabHost) NewLab(ctx context.Context, isVPN int32) (Lab, error) {
	env := newEnvironment(lh.Vlib)
	if err := env.Create(ctx, isVPN); err != nil {
		return nil, fmt.Errorf("new environment create err %v ", err)
	}

	if err := env.Add(ctx, lh.Conf.Exercises...); err != nil {
		return nil, fmt.Errorf("new environment add err %v ", err)
	}
	// setting disabled exercise tags
	// will not be run when event is created
	// instead will be created
	// manual start is required

	env.SetDisabledExercises(lh.Conf.DisabledExercises)

	dockerHost := docker.NewHost()
	l := &lab{
		tag:         generateTag(),
		lib:         lh.Vlib,
		environment: env,
		dockerHost:  dockerHost,
		frontends:   map[uint]frontendConf{},
	}

	for _, f := range lh.Conf.Frontends {
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
	Suspend(context.Context) error
	Resume(context.Context) error
	Environment() exercise.Environment
	ResetFrontends(ctx context.Context, eventTag, teamId string) error
	RdpConnPorts() []uint
	Tag() string
	AddChallenge(ctx context.Context, confs ...store.Exercise) error
	InstanceInfo() []virtual.InstanceInfo
	Close() error
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

	vm, err := l.lib.GetCopy(
		ctx,
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

func (l *lab) AddChallenge(ctx context.Context, confs ...store.Exercise) error {
	var waitGroup sync.WaitGroup
	var startByTagError error
	if err := l.environment.Add(ctx, confs...); err != nil {
		return err
	}

	for _, ch := range confs {
		if ch.Static {
			// in case of no docker or vm given to start
			// skip it
			continue
		}
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if err := l.environment.StartByTag(ctx, string(ch.Tag)); err != nil {
				startByTagError = err
			}
		}()
		waitGroup.Wait()
	}

	return startByTagError
}

func (l *lab) Environment() exercise.Environment {
	return l.environment
}

func (l *lab) ResetFrontends(ctx context.Context, eventTag, teamId string) error {
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

		err = vbox.CreateFolderLink(vm.Info().Id, eventTag, teamId)
		if err != nil {
			log.Logger.Debug().Msgf("Error creating shared folder link after vm reset: %s", err)
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

	if err := l.environment.Reset(ctx); err != nil {
		return err
	}

	for _, fconf := range l.frontends {
		switch fconf.vm.Info().State {
		case virtual.Running:
			if err := fconf.vm.Stop(); err != nil {
				return err
			}
			if err := fconf.vm.Start(ctx); err != nil {
				return err
			}
		case virtual.Stopped:
			if err := fconf.vm.Start(ctx); err != nil {
				return err
			}
		case virtual.Suspended:
			if err := fconf.vm.Start(ctx); err != nil {
				return err
			}
			if err := fconf.vm.Stop(); err != nil {
				return err
			}
			if err := fconf.vm.Start(ctx); err != nil {
				return err
			}

		case virtual.Error:
			if err := fconf.vm.Create(ctx); err != nil {
				return err
			}
			if err := fconf.vm.Start(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (l *lab) Close() error {
	var wg sync.WaitGroup

	for _, lab := range l.frontends {
		wg.Add(1)
		go func(vm vbox.VM) {
			// closing VMs....
			defer wg.Done()
			if err := vm.Close(); err != nil {
				log.Error().Msgf("Error on Close function in lab.go %s", err)
			}
		}(lab.vm)
	}
	wg.Add(1)
	go func(environment exercise.Environment) {
		// closing environment containers...
		defer wg.Done()
		if err := environment.Close(); err != nil {
			log.Error().Msgf("Error while closing environment containers %s", err)
		}

	}(l.environment)
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

func (l *lab) Tag() string {
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

func (l *lab) Suspend(ctx context.Context) error {

	if err := l.environment.Suspend(ctx); err != nil {
		return err
	}

	for _, fconf := range l.frontends {
		if fconf.vm.Info().State == virtual.Running {
			if err := fconf.vm.Suspend(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (l *lab) Resume(ctx context.Context) error {
	if err := l.environment.Resume(ctx); err != nil {
		return err
	}
	for _, fconf := range l.frontends {
		state := fconf.vm.Info().State
		if state == virtual.Suspended {
			if err := fconf.vm.Start(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}
