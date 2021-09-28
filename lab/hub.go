// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package lab

import (
	"context"
	"errors"
	"sync"

	"github.com/aau-network-security/haaukins/store"
	"github.com/rs/zerolog/log"
)

var (
	ErrBufferSize = errors.New("Buffer cannot be larger than capacity")
	ErrNoLabByTag = errors.New("Could not find lab by the specified tag")
)

type Hub interface {
	Queue() <-chan Lab
	Close() error
	Suspend(context.Context) error
	Resume(context.Context) error
	Update(labTag <-chan Lab)
	Labs() map[string]Lab
	UpdateExercises(exercises []store.Exercise)
}

type hub struct {
	creator         Creator
	queue           <-chan Lab
	update          <-chan Lab
	deallocatedLabs chan<- Lab
	labs            map[string]Lab
	stop            chan struct{}
}

func NewHub(creator Creator, buffer int, cap int, isVPN int32) (*hub, error) {
	workerAmount := 2
	if buffer < workerAmount {
		buffer = workerAmount
	}
	ready := make(chan struct{})
	stop := make(chan struct{})
	labs := make(chan Lab, buffer-workerAmount)
	queue := make(chan Lab, buffer-workerAmount)
	deallocated := make(chan Lab, cap)

	var wg sync.WaitGroup
	worker := func() {
		ctx := context.Background()
		for range ready {
			wg.Add(1)
			// todo: handle this in case of error
			l, err := creator.NewLab(ctx, isVPN)
			if err != nil {
				log.Error().Msgf("Error while creating new lab %s", err.Error())
			}

			if err := l.Start(ctx); err != nil {
				log.Error().Msgf("Error while starting lab %s", err.Error())
			}
			select {
			case labs <- l:
				wg.Done()
			case <-stop:
				wg.Done()

				/* Delete lab as it wasn't added to the lab queue */
				if err := l.Close(); err != nil {
					log.Error().Msgf("Error while closing lab %s", err.Error())
				}
				break
			}
		}
	}

	for i := 0; i < workerAmount; i++ {
		go worker()
		ready <- struct{}{}
	}

	startedLabs := map[string]Lab{}
	go func() {
		permOnce := func(f func()) func() {
			var once sync.Once
			return func() {
				once.Do(f)
			}
		}

		labsCloser := permOnce(func() { close(labs) })
		readyCloser := permOnce(func() { close(ready) })

		defer readyCloser()

		for {
			select {
			case l := <-labs:
				startedLabs[l.Tag()] = l
				select {
				case queue <- l:
				case <-stop:
					continue
				}

				// minus one, as one worker is inactive
				labsStarting := len(startedLabs) + workerAmount - 1
				if labsStarting == cap {
					readyCloser()
					continue
				}

				if len(startedLabs) == cap {
					continue
				}

				ready <- struct{}{}

			case <-stop:
				// wait for workers to finish starting labs
				wg.Wait()
				// close lab chan and iterate its content
				labsCloser()
				for l := range labs {
					if err := l.Close(); err != nil {
						log.Error().Msgf("Error while closing ready labs %s", err.Error())
					}
				}

				for _, l := range startedLabs {
					if err := l.Close(); err != nil {
						log.Error().Msgf("Error while closing started labs %s", err.Error())
					}
				}
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case lb := <-deallocated:
				queue <- lb
			}
		}
	}()

	return &hub{
		creator:         creator,
		queue:           queue,
		stop:            stop,
		labs:            startedLabs,
		deallocatedLabs: deallocated,
	}, nil
}

func (h *hub) Queue() <-chan Lab {
	return h.queue
}

func (h *hub) Close() error {
	close(h.stop)
	return nil
}

func (h *hub) Update(lab <-chan Lab) {
	lb := <-lab
	h.deallocatedLabs <- lb
}

func (h *hub) Suspend(ctx context.Context) error {
	var suspendError error
	var wg sync.WaitGroup
	for _, l := range h.labs {
		wg.Add(1)
		go func() {
			if err := l.Suspend(ctx); err != nil {
				err = suspendError
			}
			wg.Done()
		}()
		wg.Wait()
	}

	return suspendError
}

func (h *hub) UpdateExercises(exercises []store.Exercise) {
	log.Debug().Msgf("[add-challenge]: Updating set of exercises on NewLab ... ")
	h.creator.UpdateExercises(exercises)
}

func (h *hub) Resume(ctx context.Context) error {

	var resumeError error
	var wg sync.WaitGroup

	for _, l := range h.labs {
		wg.Add(1)
		go func() {
			if err := l.Resume(ctx); err != nil {
				err = resumeError
			}
			wg.Done()
		}()
		wg.Wait()

	}

	return resumeError
}

func (h *hub) Labs() map[string]Lab {
	return h.labs
}
