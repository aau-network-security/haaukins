package lab

import (
	"math/rand"
	"time"

	"io"
	"sync"

	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/virtual"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
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
	NewLab(lib vbox.Library, config Config) (Lab, error)
}

type labHost struct{}

func (lh *labHost) NewLab(lib vbox.Library, config Config) (Lab, error) {
	environ, err := newEnvironment(lib, config.Exercises...)
	if err != nil {
		return nil, err
	}

	dockerHost := docker.NewHost()

	l := &lab{
		tag:         generateTag(),
		lib:         lib,
		environment: environ,
		dockerHost:  dockerHost,
	}

	for _, f := range config.Frontends {
		_, err = l.addFrontend(f)
		if err != nil {
			return nil, err
		}
	}

	l.closers = append(l.closers, environ)

	return l, nil
}

type Lab interface {
	Start() error
	Stop() error
	Restart() error
	GetEnvironment() exercise.Environment
	RdpConnPorts() []uint
	GetTag() string
	io.Closer
}

type lab struct {
	tag          string
	lib          vbox.Library
	environment  exercise.Environment
	frontends    []vbox.VM
	rdpConnPorts []uint
	dockerHost   docker.Host

	closers []io.Closer
}

func (l *lab) addFrontend(conf store.InstanceConfig) (vbox.VM, error) {
	hostIp, err := l.dockerHost.GetDockerHostIP()

	if err != nil {
		return nil, err
	}

	rdpPort := virtual.GetAvailablePort()
	vm, err := l.lib.GetCopy(conf,
		vbox.SetBridge(l.environment.NetworkInterface()),
		vbox.SetLocalRDP(hostIp, rdpPort),
	)
	if err != nil {
		return nil, err
	}
	l.frontends = append(l.frontends, vm)
	l.rdpConnPorts = append(l.rdpConnPorts, rdpPort)
	l.closers = append(l.closers, vm)

	log.Debug().Msgf("Created lab frontend on port %d", rdpPort)

	return vm, nil
}

func (l *lab) GetEnvironment() exercise.Environment {
	return l.environment
}

func (l *lab) Start() error {
	if err := l.environment.Start(); err != nil {
		return err
	}

	for _, frontend := range l.frontends {
		if err := frontend.Start(); err != nil {
			return err
		}
	}

	return nil
}

func (l *lab) Stop() error {
	if err := l.environment.Stop(); err != nil {
		return err
	}

	for _, frontend := range l.frontends {
		if err := frontend.Stop(); err != nil {
			return err
		}
	}

	return nil
}

func (l *lab) Restart() error {
	if err := l.environment.Restart(); err != nil {
		return err
	}

	for _, frontend := range l.frontends {
		if err := frontend.Restart(); err != nil {
			return err
		}
	}

	return nil
}

func (l *lab) Close() error {
	var wg sync.WaitGroup

	for _, closer := range l.closers {
		wg.Add(1)
		go func(c io.Closer) {
			if err := c.Close(); err != nil {
				log.Warn().Msgf("error while closing lab: %s", err)
			}
			wg.Done()
		}(closer)
	}
	wg.Wait()

	return nil
}

func (l *lab) RdpConnPorts() []uint {
	return l.rdpConnPorts
}

func (l *lab) GetTag() string {
	return l.tag
}

func generateTag() string {
	// seed for our GetRandomName
	rand.Seed(time.Now().UnixNano())
	return namesgenerator.GetRandomName(0)
}
