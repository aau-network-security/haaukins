// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package daemon

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"context"

	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/aau-network-security/haaukins/app/client/cli"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/event"
	"github.com/aau-network-security/haaukins/exercise"
	"github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/virtual"
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

func (a *noAuth) TokenForUser(username, password string) (string, error) {
	return respToken, nil
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

func TestInviteUser(t *testing.T) {
	tt := []struct {
		name      string
		token     string
		allowed   bool
		superuser bool
		err       string
	}{
		{name: "Normal with auth and super", allowed: true, superuser: true},
		{name: "No super with auth", allowed: true, err: "This action requires super user permissions"},
		{name: "Unauthorized", allowed: false, err: "unauthorized"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewSignupKeyStore([]store.SignupKey{})

			ctx := context.Background()
			d := &daemon{
				auth: &noAuth{
					allowed:   tc.allowed,
					superuser: tc.superuser,
				},
				users: struct {
					store.SignupKeyStore
					store.UserStore
				}{
					s,
					store.NewUserStore([]store.User{}),
				},
			}

			dialer, close := getServer(d)
			defer close()

			conn, err := grpc.DialContext(ctx, "bufnet",
				grpc.WithDialer(dialer),
				grpc.WithInsecure(),
				grpc.WithPerRPCCredentials(cli.Creds{Token: tc.token, Insecure: true}),
			)

			if err != nil {
				t.Fatalf("failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewDaemonClient(conn)
			resp, err := client.InviteUser(ctx, &pb.InviteUserRequest{})
			if resp != nil && resp.Error != "" {
				err = fmt.Errorf(resp.Error)
			}

			if err != nil {
				st, ok := status.FromError(err)
				if ok {
					err = fmt.Errorf(st.Message())
				}

				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: %s) received: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("expected no error, but received: %s", err)
			}

			if tc.err != "" {
				t.Fatalf("expected error, but received none")
			}

			if resp.Key == "" {
				t.Fatalf("expected key not be empty string")
			}

			if len(s.ListSignupKeys()) != 1 {
				t.Fatalf("expected one key to have been inserted into store")
			}

		})
	}
}

func TestSignupUser(t *testing.T) {
	tt := []struct {
		name      string
		createKey bool
		user      string
		pass      string
		err       string
	}{
		{name: "Normal", createKey: true, user: "tkp", pass: "tkptkp"},
		{name: "Too short password", createKey: true, user: "tkp", pass: "tkp", err: "Password too short, requires atleast six characters"},
		{name: "No key", user: "tkp", pass: "tkptkp", err: "Signup key not found"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var key store.SignupKey
			var signupKeys []store.SignupKey
			if tc.createKey {
				key = store.NewSignupKey()
				signupKeys = append(signupKeys, key)
			}

			ks := store.NewSignupKeyStore(signupKeys)
			us := store.NewUserStore([]store.User{})

			ctx := context.Background()
			d := &daemon{
				auth: &noAuth{
					allowed: true,
				},
				users: struct {
					store.SignupKeyStore
					store.UserStore
				}{
					ks,
					us,
				},
			}

			dialer, close := getServer(d)
			defer close()

			conn, err := grpc.DialContext(ctx, "bufnet",
				grpc.WithDialer(dialer),
				grpc.WithInsecure(),
			)

			if err != nil {
				t.Fatalf("failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewDaemonClient(conn)
			resp, err := client.SignupUser(ctx, &pb.SignupUserRequest{
				Key:      key.String(),
				Username: tc.user,
				Password: tc.pass,
			})
			if err != nil {
				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: %s) received: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("expected no error, but received: %s", err)
			}

			if resp.Error != "" {
				if tc.err != "" {
					if tc.err != resp.Error {
						t.Fatalf("unexpected response error (expected: %s) received: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("expected no response error, but received: %s", resp.Error)
			}

			if tc.err != "" {
				t.Fatalf("expected error, but received none")
			}

			if len(us.ListUsers()) != 1 {
				t.Fatalf("expected one user to have been created in store")
			}

			if resp.Token != respToken {
				t.Fatalf("unexpected token (expected: %s) in response: %s", respToken, resp.Token)
			}
		})
	}
}

func TestLoginUser(t *testing.T) {
	type user struct {
		u string
		p string
	}

	tt := []struct {
		name       string
		createUser bool
		user       user
		err        string
	}{
		{name: "Normal", createUser: true, user: user{u: "tkp", p: "tkptkp"}},
		{name: "Unknown user", user: user{u: "tkp", p: "tkptkp"}, err: "Invalid username or password"},
		{name: "No username", user: user{u: "", p: "whatever"}, err: "Username cannot be empty"},
		{name: "No password", user: user{u: "tkp", p: ""}, err: "Password cannot be empty"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var users []store.User
			if tc.createUser {
				u, err := store.NewUser(tc.user.u, tc.user.p)
				if err != nil {
					t.Fatalf("unexpected error when creating user: %s", err)
				}

				users = append(users, u)
			}

			us := store.NewUserStore(users)
			auth := NewAuthenticator(us, "some-signing-key")
			ctx := context.Background()
			d := &daemon{
				auth: auth,
				users: struct {
					store.SignupKeyStore
					store.UserStore
				}{
					nil,
					us,
				},
			}

			dialer, close := getServer(d)
			defer close()

			conn, err := grpc.DialContext(ctx, "bufnet",
				grpc.WithDialer(dialer),
				grpc.WithInsecure(),
			)

			if err != nil {
				t.Fatalf("failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewDaemonClient(conn)
			resp, err := client.LoginUser(ctx, &pb.LoginUserRequest{
				Username: tc.user.u,
				Password: tc.user.p,
			})
			if err != nil {
				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: %s) received: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("expected no error, but received: %s", err)
			}

			if resp.Error != "" {
				if tc.err != "" {
					if tc.err != resp.Error {
						t.Fatalf("unexpected response error (expected: %s) received: %s", tc.err, resp.Error)
					}

					return
				}

				t.Fatalf("expected no response error, but received: %s", resp.Error)
			}

			if tc.err != "" {
				t.Fatalf("expected error, but received none")
			}

			if resp.Token == "" {
				t.Fatalf("expected token to be non-empty")
			}
		})
	}
}

type fakeEventHost struct {
	event event.Event
}

func (eh fakeEventHost) CreateEventFromConfig(store.EventConfig) (event.Event, error) {
	return eh.event, nil
}

func (eh fakeEventHost) CreateEventFromEventFile(store.EventFile) (event.Event, error) {
	return eh.event, nil
}

type fakeEvent struct {
	m         sync.Mutex
	connected int
	started   int
	close     int
	register  int
	finished  int
	teams     []store.Team
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

func (fe *fakeEvent) GetTeams() []store.Team {
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

func TestCreateEvent(t *testing.T) {
	tt := []struct {
		name         string
		event        pb.CreateEventRequest
		unauthorized bool
		err          string
	}{
		{name: "Normal", event: pb.CreateEventRequest{Name: "Test", Tag: "tst", Exercises: []string{"hb"}, Frontends: []string{"kali"}}},
		{name: "Unauthorized", unauthorized: true, event: pb.CreateEventRequest{Name: "Test", Tag: "tst", Exercises: []string{"hb"}, Frontends: []string{"kali"}}, err: "unauthorized"},
		{name: "Empty name", event: pb.CreateEventRequest{Tag: "tst", Exercises: []string{"hb"}, Frontends: []string{"kali"}}, err: "Name cannot be empty for Event"},
		{name: "Empty tag", event: pb.CreateEventRequest{Name: "Test", Exercises: []string{"hb"}, Frontends: []string{"kali"}}, err: "Tag cannot be empty for Event"},
		{name: "Empty exercises", event: pb.CreateEventRequest{Name: "Test", Tag: "tst", Frontends: []string{"kali"}}, err: "Exercises cannot be empty for Event"},
		{name: "Empty frontends", event: pb.CreateEventRequest{Name: "Test", Tag: "tst", Exercises: []string{"hb"}}, err: "Frontends cannot be empty for Event"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ev := fakeEvent{}

			ctx := context.Background()
			eventPool := NewEventPool("")
			d := &daemon{
				conf:      &Config{},
				eventPool: eventPool,
				frontends: &fakeFrontendStore{},
				auth: &noAuth{
					allowed: !tc.unauthorized,
				},
				ehost: &fakeEventHost{
					event: &ev,
				},
			}

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
			stream, err := client.CreateEvent(ctx, &tc.event)
			if err != nil {
				t.Fatalf("expected no error when initiating connection, but received: %s", err)
			}

			for {
				_, err = stream.Recv()
				if err != nil {
					break
				}
			}

			st, ok := status.FromError(err)
			if ok {
				err = fmt.Errorf(st.Message())
			}

			if err != nil && err != io.EOF {
				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: %s) received: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("expected no error, but received: %s", err)

			}

			if tc.err != "" {
				if tc.err != err.Error() {
					t.Fatalf("unexpected error (expected: %s) received: %s", tc.err, err)
				}

				return
			}

			if tc.err != "" {
				t.Fatalf("expected error, but received none")
			}

			if len(eventPool.GetAllEvents()) != 1 {
				t.Fatalf("expected one event to have been created")
			}

			ev.m.Lock()
			if ev.started != 1 {
				t.Fatalf("expected event to have been started once")
			}

			if ev.connected != 1 {
				t.Fatalf("expected event to have been connected once")
			}

			if ev.close != 0 {
				t.Fatalf("expected event to not have been closed")
			}
			ev.m.Unlock()
		})
	}
}

func TestStopEvent(t *testing.T) {
	tt := []struct {
		name         string
		unauthorized bool
		event        *pb.CreateEventRequest
		stopTag      string
		err          string
	}{
		{name: "Normal", stopTag: "tst"},
		{name: "Empty delete tag", stopTag: "", err: "Tag cannot be empty"},
		{name: "Unknown tag", stopTag: "some-other-tag", err: "Unable to find event by that tag"},
		{name: "Unauthorized", unauthorized: true, stopTag: "tst", err: "unauthorized"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ev := fakeEvent{
				conf: store.EventConfig{
					Tag: store.Tag("tst"),
				},
			}

			ctx := context.Background()
			eventPool := NewEventPool("")
			d := &daemon{
				conf:      &Config{},
				eventPool: eventPool,
				auth: &noAuth{
					allowed: !tc.unauthorized,
				},
				ehost: &fakeEventHost{
					event: &ev,
				},
			}

			if err := d.createEvent(&ev); err != nil {
				t.Fatalf("expected no error when adding event")
			}

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
			stream, err := client.StopEvent(ctx, &pb.StopEventRequest{
				Tag: tc.stopTag,
			})
			if err != nil {
				t.Fatalf("expected no error when initiating connection, but received: %s", err)
			}

			for {
				_, err = stream.Recv()
				if err != nil {
					break
				}
			}

			st, ok := status.FromError(err)
			if ok {
				err = fmt.Errorf(st.Message())
			}

			if err != nil && err != io.EOF {
				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: %s) received: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("expected no error, but received: %s", err)

			}

			if tc.err != "" {
				if tc.err != err.Error() {
					t.Fatalf("unexpected error (expected: %s) received: %s", tc.err, err)
				}

				return
			}

			if tc.err != "" {
				t.Fatalf("expected error, but received none")
			}

			if len(eventPool.GetAllEvents()) != 0 {
				t.Fatalf("expected one event to have been stopped")
			}

			if ev.close != 1 {
				t.Fatalf("expected event to not have been closed")
			}

		})
	}
}

func TestListEvents(t *testing.T) {
	tt := []struct {
		name         string
		unauthorized bool
		count        int
		err          string
	}{
		{name: "Normal", count: 1},
		{name: "Normal three events", count: 3},
		{name: "Unauthorized", unauthorized: true, count: 1, err: "unauthorized"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ev := &fakeEvent{}

			ctx := context.Background()
			eventPool := NewEventPool("")
			d := &daemon{
				conf:      &Config{},
				eventPool: eventPool,
				auth: &noAuth{
					allowed: !tc.unauthorized,
				},
				ehost: &fakeEventHost{
					event: ev,
				},
			}

			for i := 1; i <= tc.count; i++ {
				tempEvent := *ev
				tempEvent.conf = store.EventConfig{Tag: store.Tag(fmt.Sprintf("tst-%d", i))}
				if err := d.createEvent(&tempEvent); err != nil {
					t.Fatalf("expected no error when adding event")
				}
			}

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
			resp, err := client.ListEvents(ctx, &pb.ListEventsRequest{})
			if err != nil {
				st, ok := status.FromError(err)
				if ok {
					err = fmt.Errorf(st.Message())
				}

				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: %s) received: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("expected no error, but received: %s", err)
			}

			if tc.err != "" {
				if tc.err != err.Error() {
					t.Fatalf("unexpected error (expected: %s) received: %s", tc.err, err)
				}

				return
			}

			if tc.err != "" {
				t.Fatalf("expected error, but received none")
			}

			if n := len(resp.Events); n != tc.count {
				t.Fatalf("unexpected amount of events (expected: %d), received: %d", tc.count, n)
			}
		})
	}
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

			ev := fakeEvent{conf: store.EventConfig{Tag: store.Tag(tc.tag)}, teams: []store.Team{}, lab: &fakeLab{environment: &fakeEnvironment{}}}
			for i := 0; i < tc.nExpectedTeams; i++ {
				g := store.Team{}
				ev.teams = append(ev.teams, g)
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

func TestResetExercise(t *testing.T) {
	tt := []struct {
		name         string
		unauthorized bool
		extag        string
		evtag        string
		teams        []*pb.Team
		err          string
		expected     int
	}{
		{
			name:     "Reset specific team",
			extag:    "sql",
			evtag:    "tst",
			teams:    []*pb.Team{{Id: "team-1"}},
			expected: 1,
		},
		{
			name:     "Reset all teams",
			extag:    "sql",
			evtag:    "tst",
			teams:    nil,
			expected: 2,
		},
		{
			name:         "Unauthorized",
			extag:        "sql",
			evtag:        "tst",
			teams:        []*pb.Team{{Id: "team-1"}},
			unauthorized: true,
			err:          "unauthorized",
		},
		{
			name:  "Unknown event",
			extag: "sql",
			evtag: "unknown",
			teams: []*pb.Team{{Id: "team-1"}},
			err:   UnknownEventErr.Error(),
		},
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

			ev := &fakeEvent{conf: store.EventConfig{Tag: store.Tag("tst")}, lab: &fakeLab{environment: &fakeEnvironment{}}}
			for i := 1; i <= 2; i++ {
				g := store.Team{Id: fmt.Sprintf("team-%d", i)}
				ev.teams = append(ev.teams, g)
			}
			eventPool.AddEvent(ev)

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
			stream, err := client.ResetExercise(ctx, &pb.ResetExerciseRequest{
				ExerciseTag: tc.extag,
				EventTag:    tc.evtag,
				Teams:       tc.teams,
			})
			if err != nil {
				t.Fatalf("expected no error when initiating connection, but received: %s", err)
			}

			count := 0
			for {
				_, err := stream.Recv()
				if err != nil {
					break
				}
				count += 1
			}

			if err != nil && err != io.EOF {
				st, ok := status.FromError(err)
				if ok {
					err = fmt.Errorf(st.Message())
				}
				if tc.err != "" && err.Error() != tc.err {
					t.Fatalf("expected error '%s', but got '%s'", tc.err, err.Error())
				}
				return
			}

			if count != tc.expected {
				t.Fatalf("Expected %d resets, but observed %d", tc.expected, count)
			}
		})
	}
}

func TestListFrontends(t *testing.T) {
	tt := []struct {
		name           string
		unauthorized   bool
		err            string
		expectedImages []string
	}{
		{
			name:           "Normal",
			expectedImages: []string{"1/1", "2/2"},
		},
		{
			name:         "Unauthorized",
			unauthorized: true,
			err:          "unauthorized",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("failed to create temporary directory")
			}
			defer os.RemoveAll(tmpDir)
			for _, dir := range []string{"1", "2"} {
				if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
					t.Fatalf("failed to created subdirectories")
				}
			}

			data := "content of file"
			filenames := []string{
				filepath.Join(tmpDir, "1", "1.ova"),
				filepath.Join(tmpDir, "1", "1.txt"),
				filepath.Join(tmpDir, "2", "2.ova"),
			}
			for _, fn := range filenames {
				f, err := os.Create(fn)
				if err != nil {
					t.Fatalf("failed to create '%s': %s", fn, err)
				}
				defer f.Close()
				if _, err := f.WriteString(data); err != nil {
					t.Fatalf("failed to write to '%s': %s", fn, err)
				}
			}

			ctx := context.Background()

			d := &daemon{
				conf: &Config{
					OvaDir: tmpDir,
				},
				eventPool: NewEventPool(""),
				frontends: &fakeFrontendStore{},
				auth: &noAuth{
					allowed: !tc.unauthorized,
				},
			}

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
			resp, err := client.ListFrontends(ctx, &pb.Empty{})

			if err != nil && err != io.EOF {
				st, ok := status.FromError(err)
				if ok {
					err = fmt.Errorf(st.Message())
				}
				if tc.err != "" && err.Error() != tc.err {
					t.Fatalf("expected error '%s', but got '%s'", tc.err, err.Error())
				}
				return
			}

			if len(resp.Frontends) != len(tc.expectedImages) {
				t.Fatalf("expected %d frontends, but got %d", len(tc.expectedImages), len(resp.Frontends))
			}

			for i, f := range resp.Frontends {
				if f.Image != tc.expectedImages[i] {
					t.Fatalf("expected image '%s', but got '%s'", tc.expectedImages[i], f.Image)
				}
			}
		})
	}
}

func TestGetTeamInfo(t *testing.T) {
	tt := []struct {
		name         string
		unauthorized bool
		eventTag     string
		teamId       string
		err          string
		numInstances int
	}{
		{
			name:         "Normal",
			eventTag:     "existing-event",
			teamId:       "existing-team",
			numInstances: 2,
		},
		{
			name:         "Unauthorized",
			unauthorized: true,
			err:          "unauthorized",
		},
		{
			name:     "Unknown event",
			eventTag: "unknown-event",
			teamId:   "existing-team",
			err:      UnknownEventErr.Error(),
		},
		{
			name:     "Unknown team",
			eventTag: "existing-event",
			teamId:   "unknown-team",
			err:      UnknownTeamErr.Error(),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			lab := fakeLab{
				instances: []virtual.InstanceInfo{
					{"image-1", "docker", "id-1", virtual.Running},
					{"image-2", "vbox", "id-2", virtual.Running},
				},
			}
			ev := &fakeEvent{
				conf: store.EventConfig{
					Tag: "existing-event",
				},
			}
			if tc.teamId == "existing-team" {
				ev.lab = &lab
			}
			ep := NewEventPool("")
			ep.AddEvent(ev)

			d := &daemon{
				eventPool: ep,
				auth: &noAuth{
					allowed: !tc.unauthorized,
				},
			}

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
			req := &pb.GetTeamInfoRequest{
				TeamId:   tc.teamId,
				EventTag: tc.eventTag,
			}
			resp, err := client.GetTeamInfo(ctx, req)

			if err != nil && err != io.EOF {
				st, ok := status.FromError(err)
				if ok {
					err = fmt.Errorf(st.Message())
				}
				if tc.err != "" && err.Error() != tc.err {
					t.Fatalf("expected error '%s', but got '%s'", tc.err, err.Error())
				}
				return
			}

			if len(resp.Instances) != tc.numInstances {
				t.Fatalf("expected %d instances, but got %d", tc.numInstances, len(resp.Instances))
			}
		})
	}
}
