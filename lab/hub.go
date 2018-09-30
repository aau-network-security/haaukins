package lab

import (
	"errors"
	"sync"

	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/rs/zerolog/log"
)

var (
	BufferMaxRatioErr  = errors.New("Buffer cannot be larger than maximum")
	MaximumLabsErr     = errors.New("Maximum amount of labs reached")
	CouldNotFindLabErr = errors.New("Could not find lab by the specified tag")

	vboxNewLibrary = vbox.NewLibrary
)

type Hub interface {
	Get() (Lab, error)
	Close()
	Available() int
	Flags() []exercise.FlagConfig
	GetLabs() []Lab
	GetLabByTag(tag string) (Lab, error)
	//Config() []exercise.Config
}

type hub struct {
	vboxLib vbox.Library
	conf    LabConfig

	m           sync.Mutex
	createSema  *semaphore
	maximumSema *semaphore

	labs   []Lab
	buffer chan Lab
}

func NewHub(conf LabConfig, vboxLib vbox.Library, cap int, buf int) (Hub, error) {
	if buf > cap {
		return nil, BufferMaxRatioErr
	}

	createLimit := 3
	h := &hub{
		labs:        []Lab{},
		conf:        conf,
		createSema:  NewSemaphore(createLimit),
		maximumSema: NewSemaphore(cap),
		buffer:      make(chan Lab, buf),
		vboxLib:     vboxLib,
	}

	log.Debug().Msgf("Instantiating %d lab(s)", buf)
	for i := 0; i < buf; i++ {
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

	lab, err := NewLab(h.vboxLib, h.conf)
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

func (h *hub) Flags() []exercise.FlagConfig {
	return h.conf.Flags()
}

func (h *hub) GetLabs() []Lab {
	return h.labs
}

func (h *hub) GetLabByTag(tag string) (Lab, error) {
	for _, lab := range h.labs {
		if tag == lab.GetTag() {
			return lab, nil
		}
	}
	return nil, CouldNotFindLabErr
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
