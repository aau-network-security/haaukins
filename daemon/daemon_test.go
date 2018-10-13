package daemon

import (
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"context"

	"github.com/aau-network-security/go-ntp/app/client/cli"
	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/aau-network-security/go-ntp/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

type noAuth struct {
	allowed bool
}

func (a *noAuth) TokenForUser(username, password string) (string, error) {
	return "whatever", nil
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
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()

	dialer := func(string, time.Duration) (net.Conn, error) {
		return lis.Dial()
	}

	return dialer, lis.Close
}

func TestInviteUser(t *testing.T) {
	tt := []struct {
		name  string
		token string
		count int
	}{
		{name: "Normal invitation", token: "", count: 1},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			s := store.NewSignupKeyStore([]store.SignupKey{})

			ctx := context.Background()
			d := &daemon{
				auth: &noAuth{
					allowed: false,
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
				grpc.WithPerRPCCredentials(cli.Token(tc.Token)),
			)

			if err != nil {
				t.Fatalf("failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewDaemonClient(conn)
			_, err = client.InviteUser(ctx, &pb.InviteUserRequest{})
			if err != nil {
				t.Fatalf("failed to invite user: %v", err)
			}

		})
	}
}

// func TestSignupUser(t *testing.T) {
// 	cases := []struct {
// 		name     string
// 		req      pb.SignupUserRequest
// 		expected pb.LoginUserResponse
// 	}{
// 		{
// 			"OK",
// 			pb.SignupUserRequest{
// 				Key:      "keyval",
// 				Username: "user",
// 				Password: "pass",
// 			},
// 			pb.LoginUserResponse{
// 				Token: "token",
// 				Error: "",
// 			},
// 		},
// 		{
// 			"Invalid signup key",
// 			pb.SignupUserRequest{
// 				Key:      "invalid-key",
// 				Username: "user",
// 				Password: "pass",
// 			},
// 			pb.LoginUserResponse{
// 				Token: "",
// 				Error: "failure",
// 			},
// 		},
// 	}
// 	for _, c := range cases {
// 		t.Run(c.name, func(t *testing.T) {
// 			ctx := context.Background()
// 			conn, err := grpc.Dial("bufnet", grpc.WithDialer(bufDialer), grpc.WithInsecure())
// 			if err != nil {
// 				t.Fatalf("Failed to dial bufnet: %v", err)
// 			}
// 			defer conn.Close()

// 			client := pb.NewDaemonClient(conn)
// 			resp, _ := client.SignupUser(ctx, &c.req)
// 			if resp.Error != c.expected.Error {
// 				t.Fatalf("Expected error '%s', but got '%s'", c.expected.Error, resp.Error)
// 			}
// 			if resp.Token != c.expected.Token {
// 				t.Fatalf("Expected token '%s', but got '%s'", c.expected.Token, resp.Token)
// 			}
// 		})
// 	}
// }

// func TestLoginUser(t *testing.T) {
// 	cases := []struct {
// 		name     string
// 		req      pb.LoginUserRequest
// 		expected pb.LoginUserResponse
// 	}{
// 		{
// 			"OK",
// 			pb.LoginUserRequest{
// 				Username: "user",
// 				Password: "pass",
// 			},
// 			pb.LoginUserResponse{
// 				Token: "token",
// 				Error: "",
// 			},
// 		},
// 		{
// 			"Invalid credentials",
// 			pb.LoginUserRequest{
// 				Username: "user",
// 				Password: "invalid-password",
// 			},
// 			pb.LoginUserResponse{
// 				Token: "",
// 				Error: "failure",
// 			},
// 		},
// 	}
// 	for _, c := range cases {
// 		t.Run(c.name, func(t *testing.T) {
// 			ctx := context.Background()
// 			conn, err := grpc.Dial("bufnet", grpc.WithDialer(bufDialer), grpc.WithInsecure())
// 			if err != nil {
// 				t.Fatalf("Failed to dial bufnet: %v", err)
// 			}
// 			defer conn.Close()

// 			client := pb.NewDaemonClient(conn)

// 			resp, _ := client.LoginUser(ctx, &c.req)
// 			if resp.Error != c.expected.Error {
// 				t.Fatalf("Expected error '%s', but got '%s'", c.expected.Error, resp.Error)
// 			}
// 			if resp.Token != c.expected.Token {
// 				t.Fatalf("Expected token '%s', but got '%s'", c.expected.Token, resp.Token)
// 			}
// 		})
// 	}
// }

// type createEventServer struct {
// 	pb.Daemon_CreateEventServer
// }

// type testEvent struct {
// 	started bool
// 	conf    event.Config
// 	groups  []event.Group
// 	event.Event
// }

// func (ev *testEvent) Start(context.Context) error {
// 	ev.started = true
// 	return nil
// }

// func (ev *testEvent) Connect(*mux.Router) {}

// func (ev *testEvent) Close() { ev.started = false }

// func (ev *testEvent) GetConfig() event.Config {
// 	return ev.conf
// }

// func (ev *testEvent) GetGroups() []event.Group {
// 	return ev.groups
// }

// type testEventHost struct {
// 	ev event.Event
// 	EventHost
// }

// func (eh *testEventHost) CreateEvent(event.Config) (event.Event, error) {
// 	return eh.ev, nil
// }

// func TestCreateEvent(t *testing.T) {
// 	ev := &testEvent{started: false}

// 	d := daemon{
// 		conf:   &Config{Host: "localhost"},
// 		mux:    mux.NewRouter(),
// 		events: make(map[string]event.Event),
// 		eh:     &testEventHost{ev: ev},
// 	}
// 	req := pb.CreateEventRequest{
// 		Name:      "Event 1",
// 		Tag:       "ev1",
// 		Frontends: []string{"frontend1"},
// 	}

// 	resp := createEventServer{}
// 	err := d.CreateEvent(&req, &resp)
// 	if err != nil {
// 		t.Fatalf("Unexpected error: %v", err)
// 	}
// 	expectedEvents := 1
// 	if len(d.events) != expectedEvents {
// 		t.Fatalf("Expected %d event, got %d", expectedEvents, len(d.events))
// 	}
// 	time.Sleep(1 * time.Millisecond) // wait for goroutine to finish
// 	if !ev.started {
// 		t.Fatalf("Expected event to be started, but it is not")
// 	}
// }

// type stopEventServer struct {
// 	pb.Daemon_StopEventServer
// }

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
