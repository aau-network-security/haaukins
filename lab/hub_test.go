// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package lab

import (
	"context"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/aau-network-security/haaukins/exercise"
	"github.com/aau-network-security/haaukins/virtual"
	"github.com/google/uuid"
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

func (tl *testLab) Stop() error {
	return nil
}

func (tl *testLab) Restart(context.Context) error {
	return nil
}

func (tl *testLab) Environment() exercise.Environment {
	return nil
}

func (tl *testLab) ResetFrontends(context.Context) error {
	return nil
}

func (tl *testLab) Tag() string {
	return uuid.New().String()
}

func (tl *testLab) InstanceInfo() []virtual.InstanceInfo {
	return nil
}

func (tl *testLab) RdpConnPorts() []uint {
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
		read int
	}{
		{
			name: "Normal",
			buf:  5,
			cap:  10,
			read: 3,
		},
		{
			name: "Normal (max cap)",
			buf:  5,
			cap:  10,
			read: 10,
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

			startedLabsForBuffer := readAmountChan(started, tc.buf, time.Second)
			if startedLabsForBuffer != tc.buf {
				t.Fatalf("expected %d to be available, but %d are started", tc.buf, startedLabsForBuffer)
			}

			for i := 0; i < tc.read; i++ {
				<-h.Queue()
			}

			maxStarts := tc.cap - tc.buf
			startsAfterConsuming := MinInt(tc.read, maxStarts)
			startedLabsAfterQueue := readAmountChan(started, startsAfterConsuming, time.Second)
			if startedLabsAfterQueue != startsAfterConsuming {
				t.Fatalf("expected %d to be started, after fetching entire queue, but %d are started", startsAfterConsuming, startedLabsAfterQueue)
			}

			if tc.read+tc.buf >= tc.cap {
				_, ok := <-h.Queue()
				if ok {
					t.Fatalf("expected queue to be closed")
				}
			}

			if err := h.Close(); err != nil {
				t.Fatalf("expected error to be nil, but received: %s", err)
			}

			expectedClosedLabs := MinInt(tc.buf+tc.read, tc.cap)
			closedLabsAfterQueue := readAmountChan(closed, expectedClosedLabs, time.Second)
			if closedLabsAfterQueue != expectedClosedLabs {
				t.Fatalf("expected %d to be closed, after stopping lab hub, but %d were closed", expectedClosedLabs, closedLabsAfterQueue)
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

func MinInt(i, j int) int {
	return int(math.Min(float64(i), float64(j)))
}
