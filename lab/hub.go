// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package lab

import (
	"context"
	"github.com/aau-network-security/haaukins/logging"
	cError "github.com/aau-network-security/haaukins/errors"
	"github.com/sirupsen/logrus"
	"sync"
)

var (
	ErrBufferSize = "Buffer cannot be larger than capacity"
	ErrNoLabByTag = "Could not find lab by the specified tag"
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

func NewHub(ctx context.Context,creator Creator, buffer int, cap int) (*hub, error) {
	const fCall cError.FCall =  "hub.NewHub"
	workerAmount := 2
	if buffer < workerAmount {
		buffer = workerAmount
	}
	grpcLogger:=logging.LoggerFromCtx(ctx)

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
				logrus.WithFields(logrus.Fields{"err":cError.E(fCall,err)}).Error("Error while creating new lab")
			}

			if err := lab.Start(ctx); err != nil {
				logrus.WithFields(logrus.Fields{
					"err": cError.E(fCall,err),
				}).Error("Error while starting lab")
			}
			select {
			case labs <- lab:
				wg.Done()
			case <-stop:
				wg.Done()

				/* Delete lab as it wasn't added to the lab queue */
				if err := lab.Close(); err != nil {
					logrus.WithFields(logrus.Fields{
						"err": cError.E(fCall,err),
					}).Error("Error while closing lab")
				}
				break
			}
			if grpcLogger != nil {
				msg := ""
				if err := grpcLogger.Msg(msg); err != nil {
					logrus.SetFormatter(&logrus.JSONFormatter{})
					logrus.WithFields(logrus.Fields{
						"err": cError.E(fCall,err,logrus.ErrorLevel),
					}).Error("Failed to send data over grpc stream ")
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
						logrus.WithFields(logrus.Fields{
							"err":cError.E(fCall,err),
						}).Error("Error while closing ready labs")

					}
				}

				for _, l := range startedLabs {
					if err := l.Close(); err != nil {
						logrus.WithFields(logrus.Fields{"err":cError.E(fCall,err)}).Error("Error while closing started labs")
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
	const fCall cError.FCall = "hub.GetLabByTag"
	lab, ok := h.labs[t]
	if !ok {
		logrus.WithFields(logrus.Fields{"err":cError.E(fCall,ErrNoLabByTag)}).Error()
		return nil, cError.E(fCall,ErrNoLabByTag)
	}
	return lab, nil
}
