package lab

import (
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"testing"
)

var (
	env = testEnvironment{}
	lib = testLibrary{}
	v   = testVbox{}
)

type testEnvironment struct{}

func (testEnvironment) Add(conf exercise.Config, updateDNS bool) error { return nil }

func (testEnvironment) ResetByTag(t string) error { return nil }

func (testEnvironment) Interface() string { return "" }

func (testEnvironment) Kill() error { return nil }

type testLibrary struct{}

func (testLibrary) GetCopy(string, ...vbox.VMOpt) (vbox.VM, error) { return v, nil }

type testVbox struct{}

func (testVbox) Kill() error { return nil }

func (testVbox) LinkedClone(string, ...vbox.VMOpt) (vbox.VM, error) { return nil, nil }

func (testVbox) Snapshot(string) error { return nil }

func (testVbox) Start() error { return nil }

type testLab struct {
	started bool
}

func (lab *testLab) Kill() { lab.started = false }

func (lab *testLab) Exercises() exercise.Environment { return nil }

func (lab *testLab) RdpConnPorts() []uint { return nil }

func TestNewHub(t *testing.T) {
	newEnvironment = func(exercises ...exercise.Config) (exercise.Environment, error) {
		return env, nil
	}
	vboxNewLibrary = func(pwd string) vbox.Library {
		return lib
	}

	config, _ := LoadConfig("test_resources/test_lab.yml")
	bufferSize := 1
	hInterface, err := NewHub(uint(bufferSize), 2, *config, "/tmp")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

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

	config, _ := LoadConfig("test_resources/test_lab.yml")
	hInterface, _ := NewHub(1, 2, *config, "/tmp")
	h := hInterface.(*hub)

	if h.Available() != 1 {
		t.Fatalf("Unexpected number of buffered labs (%d), expected %d", hInterface.Available(), 1)
	}

	_, err := h.Get()
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

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

	config, _ := LoadConfig("test_resources/test_lab.yml")
	hInterface, _ := NewHub(1, 2, *config, "/tmp")
	h := hInterface.(*hub)

	h.labs = make(map[string]Lab)
	h.labs["a"] = &testLab{true}
	h.labs["b"] = &testLab{true}

	h.Close()

	for _, lab := range h.labs {
		if lab.(*testLab).started {
			t.Fatalf("Lab was expected to be killed, but is still running")
		}
	}
}
