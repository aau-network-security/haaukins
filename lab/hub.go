// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package lab

import (
	"context"
	"errors"
	"github.com/aau-network-security/haaukins/daemon"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/rs/zerolog/log"
	"io"
	"sync"
	"sync/atomic"
)

var (
	AvailableSizeErr   = errors.New("Available cannot be larger than capacity")
	MaximumLabsErr     = errors.New("Maximum amount of labs reached")
	CouldNotFindLabErr = errors.New("Could not find lab by the specified tag")
)

const BUFFERSIZE = 5

type Hub interface {
	Get() (Lab, error)
	Available() int32
	Flags() []store.FlagConfig
	GetLabs() []Lab
	GetLabByTag(tag string) (Lab, error)
	io.Closer
}

type hub struct {
	vboxLib vbox.Library
	conf    Config
	labHost LabHost

	m           sync.Mutex
	createSema  *semaphore
	maximumSema *semaphore
	ctx         context.Context

	labs     []Lab
	buffer   chan Lab
	numbLabs int32
}

func loggerFromCtx(ctx context.Context) *daemon.GrpcLogger {
	val := ctx.Value("grpc_logger")
	if val == nil {
		return nil
	}

	l, ok := val.(daemon.GrpcLogger)
	if !ok {
		return nil
	}

	return &l
}

func NewHub(ctx context.Context, conf Config, vboxLib vbox.Library, available int, cap int) (Hub, error) {
	if available > cap {
		return nil, AvailableSizeErr
	}

	createLimit := 3
	h := &hub{
		labs:        []Lab{},
		conf:        conf,
		createSema:  newSemaphore(createLimit),
		maximumSema: newSemaphore(cap),
		ctx:         context.Background(),
		buffer:      make(chan Lab, available),
		vboxLib:     vboxLib,
		labHost:     &labHost{},
	}

	grpcLogger := loggerFromCtx(ctx)
	log.Debug().Msgf("Instantiating %d lab(s)", available)
	for i := 0; i < available; i++ {
		go func() {
			err := h.addLab()
			if grpcLogger != nil {
				msg := ""
				if err != nil {
					msg = err.Error()
				}
				if err := grpcLogger.Msg(msg); err != nil {
					log.Debug().Msgf("failed to send data over grpc stream: %s", err)
				}
			}
			if err != nil {
				log.Warn().Msgf("error while adding lab: %s", err)
			}
		}()
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

	lab, err := h.labHost.NewLab(h.ctx, h.vboxLib, h.conf)
	if err != nil {
		log.Debug().Msgf("Error while creating new lab: %s", err)
		return err
	}

	if err := lab.Start(h.ctx); err != nil {
		log.Warn().Msgf("Error while starting lab: %s", err)
		go func(lab Lab) {
			if err := lab.Close(); err != nil {
				log.Warn().Msgf("Error while closing lab: %s", err)
			}
		}(lab)
		return err
	}

	select {
	case h.buffer <- lab:
		atomic.AddInt32(&h.numbLabs, 1)
	default:
		// sending on closed channel
	}

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
			go func() {
				if err := h.addLab(); err != nil {
					log.Warn().Msgf("Error while add lab: %s", err)
				}
			}()
		}
		h.labs = append(h.labs, lab)
		return lab, nil
	default:
		return nil, MaximumLabsErr
	}
}

func (h *hub) Close() error {
	_, cancel := context.WithCancel(h.ctx)
	cancel()

	close(h.buffer)

	var wg sync.WaitGroup

	for _, l := range h.labs {
		wg.Add(1)
		go func(l Lab) {
			if err := l.Close(); err != nil {
				log.Warn().Msgf("error while closing hub: %s", err)
			}
			wg.Done()
		}(l)
	}
	for l := range h.buffer {
		wg.Add(1)
		go func(l Lab) {
			if err := l.Close(); err != nil {
				log.Warn().Msgf("error while closing hub: %s", err)
			}
			wg.Done()
		}(l)
	}
	wg.Wait()
	return nil
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
