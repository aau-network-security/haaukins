// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package lab

import (
	"context"
	"testing"
	"time"

	"github.com/aau-network-security/haaukins/virtual/vbox"
)

type testLabHost struct {
	lab Lab
	LabHost
}

func (lh *testLabHost) NewLab(ctx context.Context, lib vbox.Library, config Config) (Lab, error) {
	return lh.lab, nil
}

type testLab struct {
	started bool
	closed  bool
	Lab
}

func (lab *testLab) Start(ctx context.Context) error {
	lab.started = true
	return nil
}

func (lab *testLab) Close() error {
	lab.closed = true
	return nil
}


func TestHub_addLab(t *testing.T) {
	tt := []struct {
		name         string
		capacity     int
		expectedErr  error
		expectedLabs int32
	}{
		{
			name:         "Normal",
			capacity:     1,
			expectedErr:  nil,
			expectedLabs: 1,
		},
		{
			name:         "Maximum labs reached",
			capacity:     0,
			expectedErr:  MaximumLabsErr,
			expectedLabs: 0,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ms := newSemaphore(tc.capacity)
			cs := newSemaphore(tc.capacity)
			lab := testLab{}
			lh := testLabHost{
				lab: &lab,
			}
			hub := hub{
				maximumSema: ms,
				createSema:  cs,
				labHost:     &lh,
				buffer:      make(chan Lab, tc.capacity),
			}

			if err := hub.addLab(); err != tc.expectedErr {
				t.Fatalf("Expected error %s, but got %s", tc.expectedErr, err)
			}

			if hub.Available() != tc.expectedLabs {
				t.Fatalf("Expected %d available lab(s), but is %d", tc.expectedLabs, hub.Available())
			}

			if tc.expectedErr == nil && !lab.started {
				t.Fatalf("Expected lab to be started, but it didn't")
			}
		})
	}

}

func TestHub_Get(t *testing.T) {
	tt := []struct {
		name              string
		cap               int
		start             int
		getCount          int
		expectedAvailable int32
		expectedErr       error
	}{
		{
			name:              "Normal",
			cap:               5,
			start:             5,
			getCount:          5,
			expectedAvailable: 0,
			expectedErr:       MaximumLabsErr,
		},
		{
			name:              "Buffer works",
			cap:               15,
			start:             10,
			getCount:          4,
			expectedAvailable: 6,
			expectedErr:       nil,
		},
		{
			name:              "Capacity hit",
			cap:               12,
			start:             10,
			getCount:          10,
			expectedAvailable: 2,
			expectedErr:       nil,
		},
		{
			name:              "Buffer larger than initial size",
			cap:               10,
			start:             3,
			getCount:          1,
			expectedAvailable: 3,
			expectedErr:       nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ms := newSemaphore(tc.cap)
			cs := newSemaphore(tc.cap)
			lh := testLabHost{
				lab: &testLab{},
			}
			hub := hub{
				maximumSema: ms,
				createSema:  cs,
				labHost:     &lh,
				buffer:      make(chan Lab, tc.start),
			}
			for i := 0; i < tc.start; i++ {
				hub.addLab()
			}

			for i := 0; i < tc.getCount; i++ {
				if _, err := hub.Get(); err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}
			}

			time.Sleep(1 * time.Millisecond)

			if hub.Available() != tc.expectedAvailable {
				t.Fatalf("Expected %d labs available, but go %d", tc.expectedAvailable, hub.Available())
			}

			if _, err := hub.Get(); err != tc.expectedErr {
				t.Fatalf("Expected error '%s', but got '%s'", tc.expectedErr, err)
			}

		})
	}
}


type TestLogger struct {
	count int
}

func (l *TestLogger) Msg(msg string)  {
     l.count++
}

func TestNewHub(t *testing.T){
	l := &TestLogger{1}
	ctx := context.WithValue(context.TODO(), "grpc_logger", l)
	ms := newSemaphore(5)
	cs := newSemaphore(6)

	h := &hub{
		maximumSema:ms,
		createSema:cs,
		ctx:ctx,
		buffer:make(chan Lab,5),
		vboxLib:nil,
		labHost:&testLabHost{
			lab:&testLab{},
		},
	}

	if err:=h.init(ctx,5); err!=nil {
		t.Fatalf("Something wrong with the implementation ! %d ",l.count)
	}
	if l.count != 5 {
		t.Fatalf("Something wrong with the implementation ! %d ",l.count)
	}
}


func TestHub_Close(t *testing.T) {
	ms := newSemaphore(2)
	cs := newSemaphore(3)
	hub := hub{
		maximumSema: ms,
		createSema:  cs,
		buffer:      make(chan Lab, 2),
		ctx:         context.Background(),
	}

	firstLab := testLab{}
	secondLab := testLab{}

	labs := []Lab{&firstLab, &secondLab}
	for _, l := range labs {
		lh := testLabHost{
			lab: l,
		}
		hub.labHost = &lh
		hub.addLab()
	}

	_, err := hub.Get()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if !firstLab.started {
		t.Fatalf("Expected the first lab to be started, but it isn't")
	}

	hub.Close()

	if !firstLab.closed {
		t.Fatalf("Expected the first lab to be closed, but it isn't")
	}

	if !secondLab.closed {
		t.Fatalf("Expected the second lab to be closed, but it isn't")
	}

	if len(hub.buffer) != 0 {
		t.Fatalf("Expected the hub buffer to be empty, but it isn't")
	}
}
