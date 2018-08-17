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
	ctf    = testCtfd{}
	guac   = testGuac{}
	proxy  = testProxy{}
	labhub = testLabhub{true}
)

// boilerplate interface implementations
type testCtfd struct {
	started bool
	flags   []exercise.FlagConfig
}

func (ctf *testCtfd) Start() error { ctf.started = true; return nil }

func (ctf *testCtfd) ID() string { return "1" }

func (ctf *testCtfd) ConnectProxy(revproxy.Proxy) error { return nil }

func (ctf *testCtfd) Close() { ctf.started = false }

func (ctf *testCtfd) Flags() []exercise.FlagConfig { return ctf.flags }

type testGuac struct {
	started        bool
	users          int
	rdpConnections int
}

func (guac *testGuac) ID() string { return "1" }

func (guac *testGuac) ConnectProxy(revproxy.Proxy) error { return nil }

func (guac *testGuac) Start(context.Context) error { guac.started = true; return nil }

func (guac *testGuac) CreateUser(username, password string) error { guac.users++; return nil }

func (guac *testGuac) CreateRDPConn(opts guacamole.CreateRDPConnOpts) error {
	guac.rdpConnections++
	return nil
}

func (guac *testGuac) Close() { guac.started = false }

type testProxy struct {
	started    bool
	nEndpoints int
}

func (proxy *testProxy) Start(context.Context) error {
	proxy.started = true
	return nil
}

func (proxy *testProxy) Add(docker.Identifier, string) error { return nil }

func (proxy *testProxy) Close() { proxy.started = false }

func (proxy *testProxy) NumberOfEndpoints() int { return proxy.nEndpoints }

type test_lab struct{}

func (lab *test_lab) Start() error { return nil }

func (lab *test_lab) Exercises() exercise.Environment { return nil }

func (lab *test_lab) Close() {}

func (lab *test_lab) RdpConnPorts() []uint { return []uint{1, 2} }

type testLabhub struct {
	started bool
}

func (hub *testLabhub) Get() (lab.Lab, error) { return &test_lab{}, nil }

func (hub *testLabhub) Close() { hub.started = false }

func (hub *testLabhub) Available() int { return 1 }

func getEvent() Event {
	ctfdNew = func(conf ctfd.Config) (ctfd.CTFd, error) {
		ctf = testCtfd{false, conf.Flags}
		return &ctf, nil
	}

	guacNew = func(conf guacamole.Config) (guacamole.Guacamole, error) {
		guac = testGuac{false, 0, 0}
		return &guac, nil
	}

	proxyNew = func(conf revproxy.Config, connectors ...revproxy.Connector) (revproxy.Proxy, error) {
		proxy = testProxy{false, len(connectors)}
		return &proxy, nil
	}

	labNewHub = func(buffer uint, max uint, config lab.Config, libpath string) (lab.Hub, error) {
		labhub = testLabhub{true}
		return &labhub, nil
	}

	getDockerHostIp = func() (string, error) {
		return "127.0.0.1", nil
	}

	ev, _ := New("test_resources/test_event.yml", "test_resources/test_exercises.yml")
	return ev
}

func TestNew(t *testing.T) {
	evInterface := getEvent()
	ev := evInterface.(*event)

	expected := 2
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
	if labhub.started {
		t.Fatalf("Expected LabHub to be stopped, but hasn't")
	}
}

func TestEvent_Register(t *testing.T) {
	ev := getEvent()
	ev.Start(context.TODO())
	_, err := ev.Register(Group{"newgroup1"})
	if err != nil {
		t.Fatalf("Unexpected error while registering: %s", err)
	}
	_, err = ev.Register(Group{"newgroup2"})
	if err != nil {
		t.Fatalf("Unexpected error while registering: %s", err)
	}

	expectedUsers := 2
	if guac.users != expectedUsers {
		t.Fatalf("Expected %d users, but there are %d", expectedUsers, guac.users)
	}
	expectedRdpConnections := 2
	if guac.rdpConnections != expectedRdpConnections {
		t.Fatalf("Expected %d rdp connections, but there are %d", expectedRdpConnections, guac.rdpConnections)
	}
}
