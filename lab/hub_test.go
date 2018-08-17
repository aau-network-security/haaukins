package lab

import (
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"testing"
	"time"
)

var (
	env = testEnvironment{}
	lib = testLibrary{}
	v   = testVbox{}
)

type testEnvironment struct{}

func (testEnvironment) Start() error { return nil }

func (testEnvironment) Add(conf exercise.Config, updateDNS bool) error { return nil }

func (testEnvironment) ResetByTag(t string) error { return nil }

func (testEnvironment) Interface() string { return "" }

func (testEnvironment) Close() error { return nil }

type testLibrary struct{}

func (testLibrary) GetCopy(string, ...vbox.VMOpt) (vbox.VM, error) { return v, nil }

type testVbox struct{}

func (testVbox) Stop() error { return nil }

func (testVbox) Close() error { return nil }

func (testVbox) LinkedClone(string, ...vbox.VMOpt) (vbox.VM, error) { return nil, nil }

func (testVbox) Snapshot(string) error { return nil }

func (testVbox) Start() error { return nil }

type testLab struct {
	started bool
}

func (lab *testLab) Start() error { return nil }

func (lab *testLab) Close() { lab.started = false }

func (lab *testLab) Exercises() exercise.Environment { return nil }

func (lab *testLab) RdpConnPorts() []uint { return nil }

func TestNewHub(t *testing.T) {
	newEnvironment = func(exercises ...exercise.Config) (exercise.Environment, error) {
		return env, nil
	}
	vboxNewLibrary = func(pwd string) vbox.Library {
		return lib
	}

	config, err := LoadConfig("test_resources/test_exercises.yml")
	if err != nil {
		t.Fatalf("Unexpected error while loading config: %s", err)
	}
	bufferSize := 1
	hInterface, err := NewHub(*config, "/tmp")
	if err != nil {
		t.Fatalf("Unexpected error while creating new lab hub: %s", err)
	}

	time.Sleep(1 * time.Millisecond)

	h := hInterface.(*hub)
	if h.Available() != bufferSize {
		t.Fatalf("Unexpected number of buffered labs (%d), expected %d", hInterface.Available(), bufferSize)
	}
}

func TestHub_Get(t *testing.T) {
	newEnvironment = func(exercises ...exercise.Config) (exercise.Environment, error) {
		return env, nil
	}
	vboxNewLibrary = func(pwd string) vbox.Library {
		return lib
	}

	config, err := LoadConfig("test_resources/test_exercises.yml")
	if err != nil {
		t.Fatalf("Unexpected error while loading config: %s", err)
	}
	hInterface, err := NewHub(*config, "/tmp")
	if err != nil {
		t.Fatalf("Unexpected error while creating new lab hub: %s", err)
	}
	h := hInterface.(*hub)

	time.Sleep(1 * time.Millisecond)

	if h.Available() != 1 {
		t.Fatalf("Unexpected number of buffered labs (%d), expected %d", hInterface.Available(), 1)
	}

	_, err = h.Get()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	time.Sleep(1 * time.Millisecond)

	if h.Available() != 1 {
		t.Fatalf("Unexpected number of buffered labs (%d), expected %d", hInterface.Available(), 1)
	}

	_, err = h.Get()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if h.Available() != 0 {
		t.Fatalf("Unexpected number of buffered labs (%d), expected %d", hInterface.Available(), 0)
	}

	_, err = h.Get()

	if err == nil {
		t.Fatalf("Expected error, but no error was returned")
	}
}

func TestHub_Close(t *testing.T) {
	newEnvironment = func(exercises ...exercise.Config) (exercise.Environment, error) {
		return env, nil
	}
	vboxNewLibrary = func(pwd string) vbox.Library {
		return lib
	}

	config, _ := LoadConfig("test_resources/test_exercises.yml")
	hInterface, _ := NewHub(*config, "/tmp")
	h := hInterface.(*hub)

	h.labs = []Lab{
		&testLab{true},
		&testLab{true},
	}

	h.Close()

	for _, lab := range h.labs {
		if lab.(*testLab).started {
			t.Fatalf("Lab was expected to be killed, but is still running")
		}
	}
}
