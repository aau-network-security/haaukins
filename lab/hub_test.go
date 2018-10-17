package lab

import (
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"testing"
)

type testLabHost struct {
	lab Lab
	LabHost
}

func (lh *testLabHost) NewLab(lib vbox.Library, config Config) (Lab, error) {
	return lh.lab, nil
}

type testLab struct {
	started bool
	closed  bool
	Lab
}

func (lab *testLab) Start() error {
	lab.started = true
	return nil
}

func (lab *testLab) Close() {
	lab.closed = true
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
		name        string
		labcount    int
	}{
		{
			name:     "Normal",
			labcount: 3,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ms := newSemaphore(tc.labcount)
			cs := newSemaphore(tc.labcount)
			lh := testLabHost{
				lab: &testLab{},
			}
			hub := hub{
				maximumSema: ms,
				createSema:  cs,
				labHost:     &lh,
				buffer:      make(chan Lab, tc.labcount),
			}
			for i := 0; i < tc.labcount; i++ {
				hub.addLab()
			}

			for i := 0; i < tc.labcount; i++ {
				if _, err := hub.Get(); err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}
			}

			if _, err := hub.Get(); err != MaximumLabsErr {
				t.Fatalf("Expected error '%s', but got '%s'", MaximumLabsErr, err)
			}
		})
	}
}

func TestHub_Close(t *testing.T) {
	ms := newSemaphore(2)
	cs := newSemaphore(3)
	hub := hub{
		maximumSema: ms,
		createSema:  cs,
		buffer:      make(chan Lab, 2),
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
