// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package daemon

import (
	"fmt"
	"github.com/aau-network-security/haaukins/exercise"
	"github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/event"
	"github.com/aau-network-security/haaukins/virtual"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"context"

	"github.com/aau-network-security/haaukins/client/cli"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/store"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

const (
	respToken = "whatever"
)

type noAuth struct {
	allowed   bool
	superuser bool
}

func (a *noAuth) AuthenticateContext(ctx context.Context) (context.Context, error) {
	if a.allowed {
		return context.WithValue(ctx, us{}, store.User{Username: "some_user", SuperUser: a.superuser}), nil
	}

	return ctx, fmt.Errorf("unauthorized")
}

func getServer(d *daemon) (func(string, time.Duration) (net.Conn, error), func() error) {
	const oneMegaByte = 1024 * 1024
	lis := bufconn.Listen(oneMegaByte)
	s := d.GetServer()
	pb.RegisterDaemonServer(s, d)

	go func() {
		s.Serve(lis)
	}()

	dialer := func(string, time.Duration) (net.Conn, error) {
		return lis.Dial()
	}

	return dialer, lis.Close
}

type fakeEventHost struct {
	event event.Event
	event.Host
}

func (eh fakeEventHost) CreateEventFromConfig(context.Context, store.EventConfig) (event.Event, error) {
	return eh.event, nil
}

type fakeEvent struct {
	m         sync.Mutex
	connected int
	started   int
	close     int
	register  int
	finished  int
	teams     []*store.Team
	lab       *fakeLab
	conf      store.EventConfig
	event.Event
}

func (fe *fakeEvent) Start(context.Context) error {
	fe.m.Lock()
	defer fe.m.Unlock()

	fe.started += 1
	return nil
}

func (fe *fakeEvent) Handler() http.Handler {
	fe.m.Lock()
	defer fe.m.Unlock()

	fe.connected += 1
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func (fe *fakeEvent) Close() error {
	fe.m.Lock()
	defer fe.m.Unlock()

	fe.close += 1

	return nil
}

func (fe *fakeEvent) Finish() {
	fe.m.Lock()
	defer fe.m.Unlock()

	fe.finished += 1
}

func (fe *fakeEvent) Register(store.Team) error {
	fe.m.Lock()
	defer fe.m.Unlock()

	fe.register += 1
	return nil
}

func (fe *fakeEvent) GetConfig() store.EventConfig {
	fe.m.Lock()
	defer fe.m.Unlock()

	return fe.conf
}

func (fe *fakeEvent) GetTeams() []*store.Team {
	fe.m.Lock()
	defer fe.m.Unlock()

	return fe.teams
}

func (fe *fakeEvent) GetLabByTeam(teamId string) (lab.Lab, bool) {
	if fe.lab != nil {
		return fe.lab, true
	}
	return nil, false
}

type fakeLab struct {
	environment exercise.Environment
	instances   []virtual.InstanceInfo
	lab.Lab
}

func (fl *fakeLab) Environment() exercise.Environment {
	return fl.environment
}

func (fl *fakeLab) InstanceInfo() []virtual.InstanceInfo {
	return fl.instances
}

type fakeEnvironment struct {
	resettedExercises int
	exercise.Environment
}

func (fe *fakeEnvironment) ResetByTag(ctx context.Context, t string) error {
	fe.resettedExercises += 1
	return nil
}

type fakeFrontendStore struct {
	store.FrontendStore
}

func (fe *fakeFrontendStore) GetFrontends(names ...string) []store.InstanceConfig {
	var res []store.InstanceConfig
	for _, f := range names {
		res = append(res, store.InstanceConfig{Image: f})
	}
	return res
}


func TestListEventTeams(t *testing.T) {
	tt := []struct {
		name           string
		unauthorized   bool
		tag            string
		err            string
		nExpectedTeams int
	}{
		{name: "Normal", tag: "tst", nExpectedTeams: 4},
		{name: "Unauthorized", unauthorized: true, err: "unauthorized"},
		{name: "Unknown event", tag: "unknown", err: UnknownEventErr.Error()},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			eventPool := NewEventPool("")

			d := &daemon{
				conf:      &Config{},
				eventPool: eventPool,
				auth: &noAuth{
					allowed: !tc.unauthorized,
				},
			}

			ev := fakeEvent{conf: store.EventConfig{Tag: store.Tag(tc.tag)}, teams: []*store.Team{}, lab: &fakeLab{environment: &fakeEnvironment{}}}
			for i := 0; i < tc.nExpectedTeams; i++ {
				g := store.Team{}
				ev.teams = append(ev.teams, &g)
			}
			eventPool.AddEvent(&ev)

			dialer, close := getServer(d)
			defer close()

			conn, err := grpc.DialContext(ctx, "bufnet",
				grpc.WithDialer(dialer),
				grpc.WithInsecure(),
				grpc.WithPerRPCCredentials(cli.Creds{Insecure: true}),
			)

			if err != nil {
				t.Fatalf("failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewDaemonClient(conn)
			resp, err := client.ListEventTeams(ctx, &pb.ListEventTeamsRequest{Tag: tc.tag})
			if err != nil {
				st, ok := status.FromError(err)
				if ok {
					err = fmt.Errorf(st.Message())
				}
				if err.Error() != tc.err {
					t.Fatalf("expected error '%s', but got '%s'", tc.err, err.Error())
				}
			} else {
				if len(resp.Teams) != tc.nExpectedTeams {
					t.Fatalf("expected %d teams, but got %d", tc.nExpectedTeams, len(resp.Teams))
				}
			}
		})
	}
}