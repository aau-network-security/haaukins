package lab

import (
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
)

type Lab interface {
	Kill()
	ExerEnviron() exercise.Environment
}

func NewLab(lib vbox.Library, exer []exercise.Config) (Lab, error) {
	environ, err := exercise.NewEnvironment(exer...)
	if err != nil {
		return nil, err
	}

	vm, err := lib.GetCopy("kali.ova")
	if err != nil {
		return nil, err
	}

	if err := vm.SetBridge(environ.Interface()); err != nil {
		return nil, err
	}

	return nil, nil
}

type Hub interface {
	Get() (Lab, error)
}

func NewHub(buffer int, max int) (Hub, error) {

}

type hub struct {
	labs   map[string]Lab
	buffer []Lab
}
