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
	Kill()
	Exercises() exercise.Environment
	RdpConnPorts() []uint
}

type lab struct {
	lib          vbox.Library
	exercises    exercise.Environment
	frontends    []vbox.VM
	rdpConnPorts []uint
}

func NewLab(lib vbox.Library, exer ...exercise.Config) (Lab, error) {
	environ, err := newEnvironment(exer...)
	if err != nil {
		return nil, err
	}

	l := &lab{
		lib:       lib,
		exercises: environ,
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
		vbox.SetBridge(l.exercises.Interface()),
		vbox.SetLocalRDP(rdpPort),
	)
	if err != nil {
		return nil, err
	}

	if err := vm.Start(); err != nil {
		return nil, err
	}

	l.frontends = append(l.frontends, vm)
	l.rdpConnPorts = append(l.rdpConnPorts, rdpPort)

	log.Debug().Msgf("Succesfully created frontend on port %d", rdpPort)

	return vm, nil
}

func (l *lab) Exercises() exercise.Environment {
	return l.exercises
}

func (l *lab) Kill() {
	for _, frontend := range l.frontends {
		frontend.Kill()
	}

	l.exercises.Kill()
}

func (l *lab) RdpConnPorts() []uint {
	return l.rdpConnPorts
}
