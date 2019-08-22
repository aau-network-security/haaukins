// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package lab

import (
	"context"
	"errors"

	"io"
	"sync"

	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/rs/zerolog/log"
)

var (
	AvailableSizeErr   = errors.New("Available cannot be larger than capacity")
	MaximumLabsErr     = errors.New("Maximum amount of labs reached")
	NoLabsAvailableErr = errors.New("No labs available at the moment")
	CouldNotFindLabErr = errors.New("Could not find lab by the specified tag")
)

const BUFFERSIZE = 5

type Hub interface {
	Get() (Lab, error)
	Available() int
	Flags() []store.FlagConfig
	GetLabs() []Lab
	GetLabByTag(tag string) (Lab, error)
	AttachHook(hook func() error)
	io.Closer
}

type hub struct {
	m sync.Mutex // for lab []Lab

	vboxLib vbox.Library
	conf    Config
	labHost LabHost
	labs    []Lab

	queue  chan Lab
	ready  chan struct{}
	stop   chan struct{}
	hooksC chan func() error

	hooks []func() error
}

type hubOpt func(*hub)

func WithLabHost(lh LabHost) func(*hub) {
	return func(h *hub) {
		h.labHost = lh
	}
}

func NewHub(ctx context.Context, conf Config, vboxLib vbox.Library, available int, cap int, opts ...hubOpt) (*hub, error) {
	if available > cap {
		return nil, AvailableSizeErr
	}

	var hooks []func() error
	hooksC := make(chan func() error)
	queue := make(chan Lab, available)
	ready := make(chan struct{})
	stop := make(chan struct{})

	var n int
	go func() {
		defer close(ready)

		for {
			select {
			case ready <- struct{}{}:
				n += 1
				if n == cap {
					return
				}

			case <-stop:
				return

			case hook := <-hooksC:
				hooks = append(hooks, hook)
			}
		}
	}()

	h := &hub{
		conf:    conf,
		vboxLib: vboxLib,
		labHost: &labHost{},
		hooks:   hooks,

		queue:  queue,
		ready:  ready,
		stop:   stop,
		hooksC: hooksC,
	}

	for i := 0; i < 2; i++ {
		go h.addWorker()
	}

	return h, nil
}

func (h *hub) addWorker() {
	for range h.ready {
		lab, err := h.createLab()
		if err != nil {
			continue
		}

		h.m.Lock()
		h.labs = append(h.labs, lab)
		h.m.Unlock()

		h.queue <- lab
	}
}

func (h *hub) AttachHook(hook func() error) {
	h.hooksC <- hook
}

func (h *hub) createLab() (Lab, error) {
	ctx := context.Background()
	lab, err := h.labHost.NewLab(ctx, h.vboxLib, h.conf)
	if err != nil {
		log.Debug().Msgf("Error while creating new lab: %s", err)
		return nil, err
	}

	if err := lab.Start(ctx); err != nil {
		log.Warn().Msgf("Error while starting lab: %s", err)
		if err := lab.Close(); err != nil {
			log.Warn().Msgf("Error while closing lab: %s", err)
		}
		return nil, err
	}

	if err := h.runHooks(); err != nil {
		log.Warn().Msgf("Error while running lab hooks: %s", err)
		return nil, err
	}

	return lab, nil
}

func (h *hub) Available() int {
	return len(h.queue)
}

func (h *hub) Get() (Lab, error) {
	select {
	case lab := <-h.queue:
		return lab, nil
	default:
		return nil, NoLabsAvailableErr
	}
}

func (h *hub) Close() error {
	close(h.stop)
	close(h.queue)

	var wg sync.WaitGroup
	h.m.Lock()
	for _, l := range h.labs {
		wg.Add(1)
		go func(l Lab) {
			if err := l.Close(); err != nil {
				log.Warn().Msgf("error while closing hub: %s", err)
			}
			wg.Done()
		}(l)
	}
	wg.Wait()
	h.m.Unlock()
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

func (h *hub) runHooks() error {
	for _, h := range h.hooks {
		if err := h(); err != nil {
			return err
		}
	}

	return nil
}
