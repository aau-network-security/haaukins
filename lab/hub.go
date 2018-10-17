package lab

import (
	"errors"
	"sync"

	"sync/atomic"

	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/rs/zerolog/log"
)

var (
	AvailableSizeErr   = errors.New("Available cannot be larger than capacity")
	MaximumLabsErr     = errors.New("Maximum amount of labs reached")
	CouldNotFindLabErr = errors.New("Could not find lab by the specified tag")
)

const BUFFERSIZE = 5

type Hub interface {
	Get() (Lab, error)
	Close()
	Available() int32
	Flags() []store.FlagConfig
	GetLabs() []Lab
	GetLabByTag(tag string) (Lab, error)
}

type hub struct {
	vboxLib vbox.Library
	conf    Config
	labHost LabHost

	m           sync.Mutex
	createSema  *semaphore
	maximumSema *semaphore

	labs     []Lab
	buffer   chan Lab
	numbLabs int32
}

func NewHub(conf Config, vboxLib vbox.Library, available int, cap int) (Hub, error) {
	if available > cap {
		return nil, AvailableSizeErr
	}

	createLimit := 3
	h := &hub{
		labs:        []Lab{},
		conf:        conf,
		createSema:  newSemaphore(createLimit),
		maximumSema: newSemaphore(cap),
		buffer:      make(chan Lab, available),
		vboxLib:     vboxLib,
		labHost:     &labHost{},
	}

	log.Debug().Msgf("Instantiating %d lab(s)", available)
	for i := 0; i < available; i++ {
		go h.addLab()
	}

	return h, nil
}

func (h *hub) addLab() error {
	if h.maximumSema.available() == 0 {
		return MaximumLabsErr
	}

	h.maximumSema.claim()
	h.createSema.claim()
	defer h.createSema.release()

	lab, err := h.labHost.NewLab(h.vboxLib, h.conf)
	if err != nil {
		log.Debug().Msgf("Error while creating new lab: %s", err)
		return err
	}

	if err := lab.Start(); err != nil {
		log.Debug().Msgf("Error while starting lab: %s", err)
		return err
	}

	h.buffer <- lab
	atomic.AddInt32(&h.numbLabs, 1)

	return nil
}

func (h *hub) Available() int32 {
	return atomic.LoadInt32(&h.numbLabs)
}

func (h *hub) Get() (Lab, error) {
	select {
	case lab := <-h.buffer:
		atomic.AddInt32(&h.numbLabs, -1)
		if atomic.LoadInt32(&h.numbLabs) < BUFFERSIZE {
			go h.addLab()
		}
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

func (h *hub) Flags() []store.FlagConfig {
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

func newSemaphore(n int) *semaphore {
	c := make(chan rsrc, n)
	for i := 0; i < n; i++ {
		c <- rsrc{}
	}

	return &semaphore{
		r: c,
	}
}

func (s *semaphore) claim() rsrc {
	return <-s.r
}

func (s *semaphore) available() int {
	return len(s.r)
}

func (s *semaphore) release() {
	s.r <- rsrc{}
}
