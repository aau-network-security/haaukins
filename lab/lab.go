package lab

import (
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/virtual"
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

func NewLab(lib vbox.Library, exer ...exercise.Config) (Lab, error) {
	environ, err := newEnvironment(exer...)
	if err != nil {
		return nil, err
	}

	l := &lab{
		lib:         lib,
		environment: environ,
	}

	_, err = l.addFrontend()
	if err != nil {
		return nil, err
	}

	return l, nil
}

func (l *lab) addFrontend() (vbox.VM, error) {
	rdpPort := virtual.GetAvailablePort()
	vm, err := l.lib.GetCopy("kali.ova",
		vbox.SetBridge(l.environment.Interface()),
		vbox.SetLocalRDP(rdpPort),
	)
	if err != nil {
		return nil, err
	}

	l.frontends = append(l.frontends, vm)
	l.rdpConnPorts = append(l.rdpConnPorts, rdpPort)

	log.Debug().Msgf("Succesfully created frontend on port %d", rdpPort)

	return vm, nil
}

func (l *lab) Exercises() exercise.Environment {
	return l.environment
}

func (l *lab) Start() error {
	for _, frontend := range l.frontends {
		if err := frontend.Start(); err != nil {
			return err
		}
	}

	if err := l.environment.Start(); err != nil {
		return err
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
