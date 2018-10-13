package daemon

import (
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"context"

	"github.com/aau-network-security/go-ntp/app/client/cli"
	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/aau-network-security/go-ntp/event"
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/store"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const (
	respToken = "whatever"
)

type noAuth struct {
	allowed bool
}

func (a *noAuth) TokenForUser(username, password string) (string, error) {
	return respToken, nil
}

func (a *noAuth) AuthenticateUserByToken(t string) error {
	if a.allowed {
		return nil
	}

	return fmt.Errorf("unauthorized")
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
		name    string
		token   string
		allowed bool
		err     string
	}{
		{name: "Normal with auth", allowed: true},
		{name: "Not authed", allowed: false, err: "rpc error: code = Unknown desc = unauthorized"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewSignupKeyStore([]store.SignupKey{})

			ctx := context.Background()
			d := &daemon{
				auth: &noAuth{
					allowed: tc.allowed,
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
			if err != nil {
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
				grpc.WithPerRPCCredentials(cli.Creds{Insecure: true}),
			)

			if err != nil {
				t.Fatalf("failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewDaemonClient(conn)
			resp, err := client.SignupUser(ctx, &pb.SignupUserRequest{
				Key:      string(key),
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
				grpc.WithPerRPCCredentials(cli.Creds{Insecure: true}),
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

			if err := auth.AuthenticateUserByToken(resp.Token); err != nil {
				t.Fatalf("expected to be able to authenticate with token")
			}
		})
	}
}

type eventCreator struct {
	calls int
	conf  store.Event
	event *fakeEvent
}

func (ec *eventCreator) CreateEvent(conf store.Event) (event.Event, error) {
	ec.conf = conf
	ec.calls += 1
	return ec.event, nil
}

type fakeEvent struct {
	connected int
	started   int
	close     int
	register  int
}

func (fe *fakeEvent) Start(context.Context) error {
	fe.started += 1
	return nil
}

func (fe *fakeEvent) Connect(*mux.Router) {
	fe.connected += 1
}

func (fe *fakeEvent) Close() {
	fe.close += 1
}

func (fe *fakeEvent) Register(store.Team) (*event.Auth, error) {
	fe.register += 1
	return nil, nil
}

func (fe *fakeEvent) GetConfig() store.Event {
	return store.Event{}
}

func (fe *fakeEvent) GetTeams() []store.Team {
	return nil
}

func (fe *fakeEvent) GetHub() lab.Hub {
	return nil
}

func TestCreateEvent(t *testing.T) {
	type user struct {
		u string
		p string
	}

	tt := []struct {
		name  string
		event pb.CreateEventRequest
		err   string
	}{
		{name: "Normal", event: pb.CreateEventRequest{Name: "Test", Tag: "tst", Exercises: []string{"hb"}, Frontends: []string{"kali"}}},
		{name: "Empty name", event: pb.CreateEventRequest{Tag: "tst", Exercises: []string{"hb"}, Frontends: []string{"kali"}}, err: "Name cannot be empty"},
		{name: "Empty tag", event: pb.CreateEventRequest{Name: "Test", Exercises: []string{"hb"}, Frontends: []string{"kali"}}, err: "Tag cannot be empty"},
		{name: "Empty exercises", event: pb.CreateEventRequest{Name: "Test", Tag: "tst", Frontends: []string{"kali"}}, err: "Exercises cannot be empty"},
		{name: "Empty frontends", event: pb.CreateEventRequest{Name: "Test", Tag: "tst", Exercises: []string{"hb"}}, err: "Frontends cannot be empty"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ev := fakeEvent{}

			ctx := context.Background()
			events := map[string]event.Event{}
			d := &daemon{
				conf:   &Config{},
				events: events,
				auth: &noAuth{
					allowed: true,
				},
				mux: mux.NewRouter(),
				ehost: &eventCreator{
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
			resp, err := client.CreateEvent(ctx, &tc.event)
			if err != nil {
				t.Fatalf("expected no error when initiating connection, but received: %s", err)
			}

			for {
				_, err = resp.Recv()
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

			if len(events) != 1 {
				t.Fatalf("expected one event to have been created")
			}

		})
	}
}

// func TestStopEvent(t *testing.T) {
// 	ev1 := &testEvent{started: true}
// 	ev2 := &testEvent{started: true}

// 	d := daemon{
// 		events: map[string]event.Event{
// 			"ev1": ev1,
// 			"ev2": ev2,
// 		},
// 	}

// 	req := pb.StopEventRequest{
// 		Tag: "ev1",
// 	}

// 	resp := stopEventServer{}
// 	err := d.StopEvent(&req, &resp)
// 	if err != nil {
// 		t.Fatalf("Unexpected error: %s", err)
// 	}

// 	expectedEvents := 1
// 	if len(d.events) != expectedEvents {
// 		t.Fatalf("Expected %d event, got %d", expectedEvents, len(d.events))
// 	}
// 	for k := range d.events {
// 		if k == "ev1" {
// 			t.Fatalf("Expected ev1 to be removed from daemon, but still exists")
// 		}
// 	}
// 	if ev1.started {
// 		t.Fatalf("Expected ev1 to be closed, but it is still running")
// 	}
// 	if !ev2.started {
// 		t.Fatalf("Expected ev2 to be running, but it is closed")
// 	}
// }

// func TestListEvents(t *testing.T) {
// 	ctx := context.Background()

// 	ev := &testEvent{
// 		conf: event.Config{
// 			LabConfig: lab.LabConfig{
// 				Exercises: []exercise.Config{ // define three empty exercises
// 					{}, {}, {},
// 				},
// 			},
// 		},
// 		groups: []event.Group{ // define two empty groups
// 			{}, {},
// 		},
// 	}

// 	d := daemon{
// 		events: map[string]event.Event{
// 			"ev": ev,
// 		},
// 	}

// 	req := pb.ListEventsRequest{}

// 	resp, err := d.ListEvents(ctx, &req)
// 	if err != nil {
// 		t.Fatalf("Unexpected error: %v", err)
// 	}

// 	if len(resp.Events) != 1 {
// 		t.Fatalf("Expected %d event, got %d", 1, len(resp.Events))
// 	}
// 	if resp.Events[0].ExerciseCount != 3 {
// 		t.Fatalf("Expected %d exercises, got %d", 3, resp.Events[0].ExerciseCount)
// 	}
// 	if resp.Events[0].GroupCount != 2 {
// 		t.Fatalf("Expected %d groups, got %d", 2, resp.Events[0].GroupCount)

// 	}
// }
