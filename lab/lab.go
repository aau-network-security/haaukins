package lab

import (
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/virtual"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/rs/zerolog/log"
)

var (
	newEnvironment = exercise.NewEnvironment
)

type Lab interface {
	Start() error
	Close()
	Exercises() exercise.Environment
	RdpConnPorts() []uint
}

type lab struct {
	lib          vbox.Library
	environment  exercise.Environment
	frontends    []vbox.VM
	rdpConnPorts []uint
}

func NewLab(lib vbox.Library, config Config) (Lab, error) {
	environ, err := newEnvironment(config.Exercises...)
	if err != nil {
		return nil, err
	}

	l := &lab{
		lib:         lib,
		environment: environ,
	}

	for _, d := range config.Frontends.Details {
		_, err = l.addFrontend(d)
		if err != nil {
			return nil, err
		}
	}

	return l, nil
}

func (l *lab) addFrontend(detail detail) (vbox.VM, error) {
	hostIp, err := docker.GetDockerHostIP()

	if err != nil {
		return nil, err
	}

	rdpPort := virtual.GetAvailablePort()
	vm, err := l.lib.GetCopy(detail.OvaFile,
		vbox.SetBridge(l.environment.Interface()),
		vbox.SetLocalRDP(hostIp, rdpPort),
		vbox.SetNAT(detail.HasNat),
	)
	if err != nil {
		return nil, err
	}

	l.frontends = append(l.frontends, vm)
	l.rdpConnPorts = append(l.rdpConnPorts, rdpPort)

	log.Debug().Msgf("Created lab frontend on port %d", rdpPort)

	return vm, nil
}

func (l *lab) Exercises() exercise.Environment {
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

func (l *lab) Close() {
	for _, frontend := range l.frontends {
		frontend.Close()
	}

	l.environment.Close()
}

func (l *lab) RdpConnPorts() []uint {
	return l.rdpConnPorts
}
