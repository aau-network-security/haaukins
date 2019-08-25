// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package lab

import (
	"context"
	"errors"
	"io"
)

var (
	ErrBufferSize      = errors.New("Buffer cannot be larger than capacity")
	ErrMaxLabs         = errors.New("Maximum amount of labs reached")
	ErrNoLabsAvailable = errors.New("No labs available at the current time")
	CouldNotFindLabErr = errors.New("Could not find lab by the specified tag")
)

const BUFFERSIZE = 5

type Hub interface {
	Queue() <-chan Lab
	io.Closer
}

type hub struct {
	creator Creator
	queue   <-chan Lab
	stop    chan struct{}
}

func NewHub(creator Creator, buffer int, cap int) (Hub, error) {
	workerAmount := 2
	if buffer < workerAmount {
		buffer = workerAmount
	}

	ready := make(chan struct{})
	stop := make(chan struct{})
	labs := make(chan Lab, buffer-workerAmount)
	queue := make(chan Lab, buffer-workerAmount)

	worker := func() {
		ctx := context.Background()
		for range ready {
			lab, err := creator.NewLab(ctx)
			if err != nil {
				// log error
			}

			if err := lab.Start(ctx); err != nil {
				// log error
			}

			select {
			case labs <- lab:
			case <-stop:
				lab.Close()
			}
		}
	}

	for i := 0; i < workerAmount; i++ {
		go worker()
		ready <- struct{}{}
	}

	go func() {
		var startedLabs []Lab

		defer close(queue)
		defer close(labs)

		for {
			select {
			case lab := <-labs:
				startedLabs = append(startedLabs, lab)
				queue <- lab

				labsStarting := len(startedLabs) + workerAmount - 1
				if labsStarting == cap {
					close(ready)
					continue
				}

				if len(startedLabs) == cap {
					close(queue)
					continue
				}

				ready <- struct{}{}

			case <-stop:
				for _, l := range startedLabs {
					l.Close()
				}
				return
			}
		}
	}()

	return &hub{
		creator: creator,
		queue:   queue,
		stop:    stop,
	}, nil
}

func (h *hub) Queue() <-chan Lab {
	return h.queue
}

func (h *hub) Close() error {
	close(h.stop)
	return nil
}
