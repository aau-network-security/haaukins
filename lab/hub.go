package lab

import (
	"errors"
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"sync"
)

var (
	BufferMaxRatioErr = errors.New("Buffer cannot be larger than maximum")
	MaximumLabsErr    = errors.New("Maximum amount of labs reached")
)

type Hub interface {
	Get() (Lab, error)
	Close()
	//Occupied() int
	//Config() []exercise.Config
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

func NewHub(buffer uint, max uint, config Config) (Hub, error) {
	if buffer > max {
		return nil, BufferMaxRatioErr
	}

	createLimit := 3
	h := &hub{
		labs:        make(map[string]Lab),
		exercises:   config.Exercises,
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

func (h *hub) Close() {
	for _, v := range h.labs {
		v.Kill()
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
