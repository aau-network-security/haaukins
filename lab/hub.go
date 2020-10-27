// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package lab

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/aau-network-security/haaukins/logging"
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
}

type hub struct {
	creator Creator
	queue   <-chan Lab
	labs    map[string]Lab
	stop    chan struct{}
}

func NewHub(ctx context.Context, creator Creator, buffer int, cap int, isVPN bool) (*hub, error) {
	workerAmount := 2
	if buffer < workerAmount {
		buffer = workerAmount
	}
	grpcLogger := logging.LoggerFromCtx(ctx)

	ready := make(chan struct{})
	stop := make(chan struct{})
	labs := make(chan Lab, buffer-workerAmount)
	queue := make(chan Lab, buffer-workerAmount)

	var wg sync.WaitGroup
	worker := func() {
		ctx := context.Background()
		for range ready {
			wg.Add(1)
			var err error
			var lab Lab
			lab, err = creator.NewLab(ctx, isVPN)
			if err != nil {
				log.Error().Msgf("Error while creating new lab %s retry activated", err.Error())
				for i := 0; ; i++ {
					attempts := 10
					lab, err = creator.NewLab(ctx, isVPN)
					if err == nil {
						return
					}
					if i >= (attempts - 1) {
						log.Error().Msgf("Lab could not be initialized after %d attempts", attempts)
						break
					}
					time.Sleep(time.Second)
					log.Error().Msgf("retrying after error: %v", err)
				}
			}

			if err := lab.Start(ctx); err != nil {
				log.Error().Msgf("Error while starting lab %s", err.Error())
			}
			select {
			case labs <- lab:
				wg.Done()
			case <-stop:
				wg.Done()

				/* Delete lab as it wasn't added to the lab queue */
				if err := lab.Close(); err != nil {
					log.Error().Msgf("Error while closing lab %s", err.Error())
				}
				break
			}
			if grpcLogger != nil {
				msg := ""
				if err := grpcLogger.Msg(msg); err != nil {
					log.Debug().Msgf("failed to send data over grpc stream: %s", err)
				}
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

		queueCloser := permOnce(func() { close(queue) })
		labsCloser := permOnce(func() { close(labs) })
		readyCloser := permOnce(func() { close(ready) })

		defer queueCloser()
		defer readyCloser()

		for {
			select {
			case lab := <-labs:
				startedLabs[lab.Tag()] = lab
				select {
				case queue <- lab:
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
					queueCloser()
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

	return &hub{
		creator: creator,
		queue:   queue,
		stop:    stop,
		labs:    startedLabs,
	}, nil
}

func (h *hub) Queue() <-chan Lab {
	return h.queue
}

func (h *hub) Close() error {
	close(h.stop)
	return nil
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
