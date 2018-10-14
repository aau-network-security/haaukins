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
	"github.com/aau-network-security/go-ntp/store"
	"github.com/gorilla/mux"
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
		{name: "Unauthorized", allowed: false, err: "unauthorized"},
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

	event.Event
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

func TestCreateEvent(t *testing.T) {
	tt := []struct {
		name         string
		event        pb.CreateEventRequest
		unauthorized bool
		err          string
	}{
		{name: "Normal", event: pb.CreateEventRequest{Name: "Test", Tag: "tst", Exercises: []string{"hb"}, Frontends: []string{"kali"}}},
		{name: "Unauthorized", unauthorized: true, event: pb.CreateEventRequest{Name: "Test", Tag: "tst", Exercises: []string{"hb"}, Frontends: []string{"kali"}}, err: "unauthorized"},
		{name: "Empty name", event: pb.CreateEventRequest{Tag: "tst", Exercises: []string{"hb"}, Frontends: []string{"kali"}}, err: "Name cannot be empty"},
		{name: "Empty tag", event: pb.CreateEventRequest{Name: "Test", Exercises: []string{"hb"}, Frontends: []string{"kali"}}, err: "Tag cannot be empty"},
		{name: "Empty exercises", event: pb.CreateEventRequest{Name: "Test", Tag: "tst", Frontends: []string{"kali"}}, err: "Exercises cannot be empty"},
		{name: "Empty frontends", event: pb.CreateEventRequest{Name: "Test", Tag: "tst", Exercises: []string{"hb"}}, err: "Frontends cannot be empty"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ev := fakeEvent{}
			ec := &eventCreator{
				event: &ev,
			}

			ctx := context.Background()
			events := map[store.Tag]event.Event{}
			d := &daemon{
				conf:   &Config{},
				events: events,
				auth: &noAuth{
					allowed: !tc.unauthorized,
				},
				mux:   mux.NewRouter(),
				ehost: ec,
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

			if len(events) != 1 {
				t.Fatalf("expected one event to have been created")
			}

			if tc.event.Name != ec.conf.Name {
				t.Fatalf("name is set incorrectly (expected: %s) received: %s", tc.event.Name, ec.conf.Name)
			}

			evtag, _ := store.NewTag(tc.event.Tag)
			if evtag != ec.conf.Tag {
				t.Fatalf("tag is set incorrectly (expected: %s) received: %s", evtag, ec.conf.Tag)
			}

			if ev.started != 1 {
				t.Fatalf("expected event to have been started once")
			}

			if ev.connected != 1 {
				t.Fatalf("expected event to have been connected once")
			}

			if ev.close != 0 {
				t.Fatalf("expected event to not have been closed")
			}

		})
	}
}

func TestStopEvent(t *testing.T) {
	dummyEvent := store.Event{Name: "Test", Tag: "tst", Lab: store.Lab{Exercises: []store.Tag{"hb"}, Frontends: []string{"kali"}}}
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
			ev := fakeEvent{}
			ec := &eventCreator{
				event: &ev,
			}

			ctx := context.Background()
			events := map[store.Tag]event.Event{}
			d := &daemon{
				conf:   &Config{},
				events: events,
				auth: &noAuth{
					allowed: !tc.unauthorized,
				},
				mux:   mux.NewRouter(),
				ehost: ec,
			}

			if err := d.createEvent(dummyEvent); err != nil {
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

			if len(events) != 0 {
				t.Fatalf("expected one event to have been stopped")
			}

			if ev.close != 1 {
				t.Fatalf("expected event to not have been closed")
			}

		})
	}
}

func TestListEvents(t *testing.T) {
	dummyEvent := &store.Event{Name: "Test", Tag: "tst", Lab: store.Lab{Exercises: []store.Tag{"hb"}, Frontends: []string{"kali"}}}
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
			ev := fakeEvent{}
			ec := &eventCreator{
				event: &ev,
			}

			ctx := context.Background()
			events := map[store.Tag]event.Event{}
			d := &daemon{
				conf:   &Config{},
				events: events,
				auth: &noAuth{
					allowed: !tc.unauthorized,
				},
				mux:   mux.NewRouter(),
				ehost: ec,
			}

			for i := 1; i <= tc.count; i++ {
				tempEvent := *dummyEvent
				tempEvent.Tag, _ = store.NewTag(fmt.Sprintf("tst-%d", i))

				if err := d.createEvent(tempEvent); err != nil {
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
