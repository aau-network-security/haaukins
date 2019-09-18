// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package lab

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrBufferSize = errors.New("Buffer cannot be larger than capacity")
	ErrNoLabByTag = errors.New("Could not find lab by the specified tag")
)

type Hub interface {
	Queue() <-chan Lab
	Close() error
	GetLabByTag(string) (Lab, error)
}

type hub struct {
	creator Creator
	queue   <-chan Lab
	labs    map[string]Lab
	stop    chan struct{}
}

func NewHub(creator Creator, buffer int, cap int) (*hub, error) {
	workerAmount := 2
	if buffer < workerAmount {
		buffer = workerAmount
	}

	ready := make(chan struct{})
	stop := make(chan struct{})
	labs := make(chan Lab, buffer-workerAmount)
	queue := make(chan Lab, buffer-workerAmount)

	var wg sync.WaitGroup
	worker := func() {
		ctx := context.Background()
		for range ready {
			wg.Add(1)
			lab, err := creator.NewLab(ctx)
			if err != nil {
				// log error
			}

			if err := lab.Start(ctx); err != nil {
				// log error
			}
			wg.Done()

			labs <- lab
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
						// log error
					}
				}

				for _, l := range startedLabs {
					if err := l.Close(); err != nil {
						// log error
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

func (h *hub) GetLabByTag(t string) (Lab, error) {
	lab, ok := h.labs[t]
	if !ok {
		return nil, ErrNoLabByTag
	}

	return lab, nil
}
