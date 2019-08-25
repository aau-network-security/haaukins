// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package lab

import (
	"context"
	"sync"
	"testing"
	"time"
)

type testLab struct {
	started chan<- bool
	closed  chan<- bool
}

func (tl *testLab) Close() error {
	tl.closed <- true
	return nil
}

func (tl *testLab) Start(context.Context) error {
	tl.started <- true
	return nil
}

type testCreator struct {
	m       sync.Mutex
	lab     Lab
	started int
}

func (c *testCreator) NewLab(context.Context) (Lab, error) {
	c.m.Lock()
	c.started += 1
	c.m.Unlock()

	return c.lab, nil

}

func TestHub(t *testing.T) {
	tt := []struct {
		name string
		buf  int
		cap  int
	}{
		{
			name: "Normal",
			buf:  5,
			cap:  10,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			started := make(chan bool, 1000)
			closed := make(chan bool, 1000)

			c := &testCreator{lab: &testLab{started, closed}}
			h, err := NewHub(c, tc.buf, tc.cap)
			if err != nil {
				t.Fatalf("unable to create hub: %s", err)
			}

			startedLabs := readAmountChan(started, tc.buf, time.Second)
			if startedLabs != tc.buf {
				t.Fatalf("expected %d to be available, but %d are started", tc.buf, startedLabs)
			}

			for i := 0; i < tc.cap; i++ {
				<-h.Queue()
			}

			additionalStarts := tc.cap - tc.buf
			startedLabsAfterQueue := readAmountChan(started, additionalStarts, time.Second)
			if startedLabsAfterQueue != additionalStarts {
				t.Fatalf("expected %d to be started, after fetching entire queue, but %d are started", additionalStarts, startedLabsAfterQueue)
			}

			_, ok := <-h.Queue()
			if ok {
				t.Fatalf("expected queue to be closed")
			}

		})
	}
}

func readAmountChan(c <-chan bool, amount int, wait time.Duration) int {
	var n int

	for {
		select {
		case <-c:
			n += 1
			if n == amount {
				return n + len(c)
			}

		case <-time.After(wait):
			return n
		}

	}
}
