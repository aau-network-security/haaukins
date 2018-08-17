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

	labs   []Lab
	buffer chan Lab
}

func NewHub(config Config, libpath string) (Hub, error) {
	if config.Capacity.Buffer > config.Capacity.Max {
		return nil, BufferMaxRatioErr
	}

	createLimit := 3
	h := &hub{
		labs:        []Lab{},
		exercises:   config.Exercises,
		createSema:  NewSemaphore(createLimit),
		maximumSema: NewSemaphore(config.Capacity.Max),
		buffer:      make(chan Lab, config.Capacity.Buffer),
		vboxLib:     vboxNewLibrary(libpath),
	}

	log.Debug().Msgf("Instantiating %d lab(s)", config.Capacity.Buffer)
	for i := 0; i < config.Capacity.Buffer; i++ {
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
		log.Debug().Msgf("Error while creating new lab: %s", err)
		return err
	}

	if err := lab.Start(); err != nil {
		log.Debug().Msgf("Error while starting lab: %s", err)
		return err
	}

	h.buffer <- lab

	return nil
}

func (h *hub) Available() int {
	return len(h.buffer)
}

func (h *hub) Get() (Lab, error) {
	select {
	case lab := <-h.buffer:
		go h.addLab()
		h.labs = append(h.labs, lab)
		return lab, nil
	default:
		return nil, MaximumLabsErr
	}
}

func (h *hub) Close() {
	for _, v := range h.labs {
		v.Close()
	}
	close(h.buffer)
	for v := range h.buffer {
		v.Close()
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
