package lab

import (
	"errors"
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/rs/zerolog/log"
	"sync"
)

var (
	BufferMaxRatioErr = errors.New("Buffer cannot be larger than maximum")
	MaximumLabsErr    = errors.New("Maximum amount of labs reached")

	vboxNewLibrary = vbox.NewLibrary
)

type Hub interface {
	Get() (Lab, error)
	Close()
	Available() int
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

func NewHub(buffer uint, max uint, config Config, libpath string) (Hub, error) {
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
		vboxLib:     vboxNewLibrary(libpath),
	}

	log.Debug().Msgf("Instantiating %d lab(s)", buffer)
	errs := make(chan error)
	for i := 0; i < int(buffer); i++ {
		go h.addLab(errs)
	}
	for i := 0; i < int(buffer); i++ {
		err := <-errs
		if err != nil {
			log.Debug().Msgf("Error while adding lab: %s", err)
		}
	}

	return h, nil
}

func (h *hub) addLab(errs chan error) {
	if h.maximumSema.Available() == 0 {
		errs <- MaximumLabsErr
		return
	}

	h.maximumSema.Claim()
	h.createSema.Claim()
	defer h.createSema.Release()

	lab, err := NewLab(h.vboxLib, h.exercises...)
	if err != nil {
		errs <- err
		return
	}

	h.buffer <- lab
	errs <- nil
}

func (h *hub) Available() int {
	return len(h.buffer)
}

func (h *hub) Get() (Lab, error) {
	select {
	case lab := <-h.buffer:
		errs := make(chan error)
		go h.addLab(errs)
		err := <-errs
		if err != nil {
			log.Debug().Msgf("Error while adding lab: %s", err)
		}
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
