package event

import (
	"context"
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
	"github.com/aau-network-security/go-ntp/svcs/revproxy"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"testing"
)

var (
	ctf    = test_ctfd{false}
	guac   = test_guac{false, 0, 0}
	proxy  = test_proxy{false}
	labhub = test_labhub{}
)

// boilerplate interface implementations
type test_ctfd struct {
	started bool
}

func (ctf *test_ctfd) ID() string { return "1" }

func (ctf *test_ctfd) ConnectProxy(revproxy.Proxy) error { return nil }

func (ctf *test_ctfd) Start(context.Context) error {
	ctf.started = true
	return nil
}

func (ctf *test_ctfd) Close() { ctf.started = false }

func (ctf *test_ctfd) Flags() []exercise.FlagConfig {
	return []exercise.FlagConfig{}
}

type test_guac struct {
	started        bool
	users          int
	rdpConnections int
}

func (guac *test_guac) ID() string { return "1" }

func (guac *test_guac) ConnectProxy(revproxy.Proxy) error { return nil }

func (guac *test_guac) Start(context.Context) error {
	guac.started = true
	return nil
}

func (guac *test_guac) CreateUser(username, password string) error {
	guac.users++
	return nil
}

func (guac *test_guac) CreateRDPConn(opts guacamole.CreateRDPConnOpts) error {
	guac.rdpConnections++
	return nil
}

func (guac *test_guac) Close() {
	guac.started = false
}

type test_proxy struct {
	started bool
}

func (proxy *test_proxy) Start(context.Context) error {
	proxy.started = true
	return nil
}

func (proxy *test_proxy) Add(docker.Identifier, string) error { return nil }

func (proxy *test_proxy) Close() { proxy.started = false }

func (proxy *test_proxy) NumberOfEndpoints() int { return 1 }

type test_lab struct{}

func (lab *test_lab) Exercises() *exercise.Environment { return nil }

func (lab *test_lab) Kill() {}

func (lab *test_lab) RdpConnPorts() []uint { return []uint{1, 2} }

type test_labhub struct{}

func (hub *test_labhub) Get() (lab.Lab, error) { return &test_lab{}, nil }

func (hub *test_labhub) Close() {}

func getEvent() Event {
	ctf = test_ctfd{false}
	guac = test_guac{false, 0, 0}
	proxy = test_proxy{false}
	labhub = test_labhub{}

	ctfdFuncOld := ctfdNew
	defer func() { ctfdNew = ctfdFuncOld }()
	ctfdNew = func(conf ctfd.Config) (ctfd.CTFd, error) {
		return &ctf, nil
	}

	guacFuncOld := guacNew
	defer func() { guacNew = guacFuncOld }()
	guacNew = func(conf guacamole.Config) (guacamole.Guacamole, error) {
		return &guac, nil
	}

	proxyFuncOld := proxyNew
	defer func() { proxyNew = proxyFuncOld }()
	proxyNew = func(conf revproxy.Config, connectors ...revproxy.Connector) (revproxy.Proxy, error) {
		return &proxy, nil
	}

	labFuncOld := labNewHub
	defer func() { labNewHub = labFuncOld }()
	labNewHub = func(buffer uint, max uint, config lab.Config) (lab.Hub, error) {
		return &labhub, nil
	}

	ev, _ := New("test_resources/test_event.yml", "test_resources/test_lab.yml")
	return ev
}

func TestNew(t *testing.T) {
	expected := 2

	evInterface, _ := New("test_resources/test_event.yml", "test_resources/test_lab.yml")
	ev := evInterface.(*event)
	nEndpoints := ev.proxy.NumberOfEndpoints()
	if nEndpoints != expected {
		t.Fatalf("Unexpected number of endpoints (expected %d): %d", expected, nEndpoints)
	}

	expected = 3
	ctfdFlags := ev.ctfd.Flags()
	if len(ctfdFlags) != expected {
		t.Fatalf("Unexpected number of flags (expected %d): %d", expected, len(ctfdFlags))
	}
	expectedFlagName := "First flag"
	if ctfdFlags[0].Name != expectedFlagName {
		t.Fatalf("Unexpected flag name (expected %v): %v", expectedFlagName, ctfdFlags[0].Name)
	}
}

func TestEvent_StartAndClose(t *testing.T) {
	ev := getEvent()
	ev.Start(context.TODO())

	if !ctf.started {
		t.Fatalf("Expected CTFd to be started, but hasn't")
	}
	if !guac.started {
		t.Fatalf("Expected Guacamole to be started, but hasn't")
	}
	if !proxy.started {
		t.Fatalf("Expected Proxy to be started, but hasn't")
	}

	ev.Close()

	if ctf.started {
		t.Fatalf("Expected CTFd to be stopped, but hasn't")
	}
	if guac.started {
		t.Fatalf("Expected Guacamole to be stopped, but hasn't")
	}
	if proxy.started {
		t.Fatalf("Expected Proxy to be stopped, but hasn't")
	}
}

func TestEvent_Register(t *testing.T) {
	ev := getEvent()
	ev.Start(context.TODO())
	ev.Register(Group{"newgroup1"})
	ev.Register(Group{"newgroup2"})

	expectedUsers := 2
	if guac.users != expectedUsers {
		t.Fatalf("Expected %d users, but there are %d", expectedUsers, guac.users)
	}
	expectedRdpConnections := 2
	if guac.rdpConnections != expectedRdpConnections {
		t.Fatalf("Expected %d rdp connections, but there are %d", expectedRdpConnections, guac.rdpConnections)
	}
}
