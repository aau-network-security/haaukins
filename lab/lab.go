package lab

import (
	"errors"
	"sync"

	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/virtual"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
)

var (
	BufferMaxRatioErr = errors.New("Buffer cannot be larger than maximum")
	MaximumLabsErr    = errors.New("Maximum amount of labs reached")
)

type Lab interface {
	Kill()
	Exercises() *exercise.Environment
}

type lab struct {
	lib          vbox.Library
	exercises    *exercise.Environment
	frontends    []vbox.VM
	rdpConnPorts []uint
}

func NewLab(lib vbox.Library, exer ...exercise.Config) (Lab, error) {
	environ, err := exercise.NewEnvironment(exer...)
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

	return vm, nil
}

func (l *lab) Exercises() *exercise.Environment {
	return l.exercises
}

func (l *lab) Kill() {
	for _, frontend := range l.frontends {
		frontend.Kill()
	}

	l.exercises.Kill()
}

type Hub interface {
	Get() (Lab, error)
}

type hub struct {
	vboxLib   vbox.Library
	exercises []exercise.Config

	m           sync.Mutex
	createSema  *semaphore
	maximumSema *semaphore

	labs   map[string]Lab
	buffer chan Lab
}

func NewHub(buffer uint, max uint, exer ...exercise.Config) (Hub, error) {
	if buffer > max {
		return nil, BufferMaxRatioErr
	}

	createLimit := 3
	h := &hub{
		labs:        make(map[string]Lab),
		exercises:   exer,
		createSema:  NewSemaphore(createLimit),
		maximumSema: NewSemaphore(int(max)),
		buffer:      make(chan Lab, buffer),
	}

	for i := 0; i < int(buffer); i++ {
		go h.addLab()
	}

	return h, nil
}

func (h *hub) addLab() error {
	if h.maximumSema.Available() == 0 {
		return MaximumLabsErr
	}

	h.maximumSema.Claim()
	h.createSema.Claim()
	defer h.createSema.Release()

	lab, err := NewLab(h.vboxLib, h.exercises...)
	if err != nil {
		return err
	}

	h.buffer <- lab

	return nil
}

func (h *hub) Get() (Lab, error) {
	select {
	case lab := <-h.buffer:
		return lab, nil
	default:
		return nil, MaximumLabsErr
	}
}

type rsrc struct{}

type semaphore struct {
	r chan rsrc
}

func NewSemaphore(n int) *semaphore {
	c := make(chan rsrc, n)
	for i := 0; i < n; i++ {
		c <- rsrc{}
	}

	return &semaphore{
		r: c,
	}
}

func (s *semaphore) Claim() rsrc {
	return <-s.r
}

func (s *semaphore) Available() int {
	return len(s.r)
}

func (s *semaphore) Release() {
	s.r <- rsrc{}
}
