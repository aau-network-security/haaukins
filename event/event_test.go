package event

import (
	"context"
	"io"
	"testing"

	"github.com/aau-network-security/haaukins/exercise"
	"github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/ctfd"
	"github.com/aau-network-security/haaukins/svcs/guacamole"
	"github.com/aau-network-security/haaukins/virtual/docker"
)

const (
	CREATED = 0
	STARTED = 1
	CLOSED  = 2
	STOPPED = 3
)

type testCtfd struct {
	status int
	ctfd.CTFd
}

func (ctf *testCtfd) Start(ctx context.Context) error {
	ctf.status = STARTED
	return nil
}

func (ctf *testCtfd) Close() error {
	ctf.status = CLOSED
	return nil
}

func (ctf *testCtfd) Stop() error {
	ctf.status = STOPPED
	return nil
}

type testGuac struct {
	status int
	guacamole.Guacamole
}

func (guac *testGuac) Start(ctx context.Context) error {
	guac.status = STARTED
	return nil
}

func (guac *testGuac) Close() error {
	guac.status = CLOSED
	return nil
}

func (guac *testGuac) CreateUser(username string, password string) error {
	return nil
}

func (guac *testGuac) CreateRDPConn(opts guacamole.CreateRDPConnOpts) error {
	return nil
}

type testEnvironment struct {
	exercise.Environment
}

func (te *testEnvironment) Challenges() []store.Challenge {
	return nil
}

type testLab struct {
	status   int
	rdpPorts []uint
	lab.Lab
}

func (lab *testLab) RdpConnPorts() []uint {
	return lab.rdpPorts
}

func (lab *testLab) GetEnvironment() exercise.Environment {
	return &testEnvironment{}
}

type testLabHub struct {
	status int
	lab    lab.Lab
	err    error
	lab.Hub
}

func (hub *testLabHub) Get() (lab.Lab, error) {
	return hub.lab, hub.err
}

func (hub *testLabHub) Close() error {
	hub.status = CLOSED
	return nil
}

type testDockerHost struct {
	docker.Host
}

func (dh *testDockerHost) GetDockerHostIP() (string, error) {
	return "1.2.3.4", nil
}

type testEventFile struct {
	store.EventFile
}

func (ef *testEventFile) GetTeams() []store.Team {
	return []store.Team{}
}

func TestEvent_StartAndClose(t *testing.T) {
	tt := []struct {
		name string
	}{
		{name: "Normal"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctfd := testCtfd{}
			guac := testGuac{}
			hub := testLabHub{}
			store := testEventFile{}

			ev := event{
				ctfd:    &ctfd,
				guac:    &guac,
				labhub:  &hub,
				closers: []io.Closer{&ctfd, &guac, &hub},
				store:   &store,
			}

			ev.Start(context.Background())

			if ctfd.status != STARTED {
				t.Fatalf("Expected CTFd to be started, but hasn't")
			}
			if guac.status != STARTED {
				t.Fatalf("Expected Guacamole to be started, but hasn't")
			}

			ev.Close()

			if ctfd.status != CLOSED {
				t.Fatalf("Expected CTFd to be stopped, but hasn't")
			}
			if guac.status != CLOSED {
				t.Fatalf("Expected Guacamole to be stopped, but hasn't")
			}
			if hub.status != CLOSED {
				t.Fatalf("Expected LabHub to be stopped, but hasn't")
			}
		})
	}
}

func TestEvent_AssignLab(t *testing.T) {
	tt := []struct {
		name         string
		hubErr       error
		expectedErr  error
		expectedLabs int
	}{
		{name: "Normal", expectedLabs: 1},
		{name: "No labs available", hubErr: lab.CouldNotFindLabErr, expectedErr: lab.CouldNotFindLabErr, expectedLabs: 0},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			l := testLab{rdpPorts: []uint{1}}
			hub := testLabHub{lab: &l, err: tc.hubErr}
			labs := make(map[string]lab.Lab)

			ev := event{
				ctfd:          &testCtfd{},
				guac:          &testGuac{},
				labhub:        &hub,
				labs:          labs,
				guacUserStore: guacamole.NewGuacUserStore(),
				dockerHost:    &testDockerHost{},
				store:         &testEventFile{},
			}
			ev.Start(context.Background())

			team := store.NewTeam("what@ever.com", "test", "passworder")
			if err := ev.AssignLab(&team); err != tc.expectedErr {
				t.Fatalf("Unexpected error %s, expected %s", err, tc.expectedErr)
			}

			if len(labs) != tc.expectedLabs {
				t.Fatalf("Expected %d lab(s) in event, but is %d", tc.expectedLabs, len(labs))
			}
		})
	}
}
