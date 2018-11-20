package lab

import (
	"context"
	"testing"

	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
)

type testDockerHost struct {
	docker.Host
}

func (dh *testDockerHost) GetDockerHostIP() (string, error) {
	return "1.2.3.4", nil
}

type testVboxLibrary struct {
	vm vbox.VM
	vbox.Library
}

func (vl *testVboxLibrary) GetCopy(context.Context, store.InstanceConfig, ...vbox.VMOpt) (vbox.VM, error) {
	return vl.vm, nil
}

type testVM struct {
	vbox.VM
}

type testEnvironment struct {
	exercise.Environment
}

func (ee *testEnvironment) NetworkInterface() string {
	return ""
}

func TestAddFrontend(t *testing.T) {
	lab := lab{
		dockerHost: &testDockerHost{},
		lib: &testVboxLibrary{
			vm: &testVM{},
		},
		environment: &testEnvironment{},
		frontends:   map[uint]vbox.VM{},
	}
	conf := store.InstanceConfig{}

	lab.addFrontend(context.Background(), conf)

	if len(lab.frontends) != 1 {
		t.Fatalf("Expected %d frontend, but is %d", len(lab.frontends), 1)
	}
}
