package daemon

import (
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"context"
	"fmt"
	"log"
	"net"
	"time"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/aau-network-security/go-ntp/event"
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/store"
	"github.com/gorilla/mux"
)

var lis *bufconn.Listener

const bufSize = 1024 * 1024 // => 1 MB

type testUserHub struct {
	signupKey string
	err       error

	username string
	password string
	token    string

	store.TeamStore
}

func (t testUserHub) CreateSignupKey() (SignupKey, error) {
	return SignupKey(t.signupKey), t.err
}

func (t testUserHub) AddUser(k SignupKey, username, password string) error {
	if k == SignupKey(t.signupKey) && username == t.username && password == t.password {
		return nil
	}
	return fmt.Errorf("failure")
}

func (t testUserHub) TokenForUser(username, password string) (string, error) {
	if username == t.username && password == t.password {
		return t.token, nil
	}
	return "", fmt.Errorf("failure")
}

func init() {
	lis = bufconn.Listen(bufSize)
	d := &daemon{
		uh: testUserHub{
			signupKey: "keyval",
			err:       nil,
			username:  "user",
			password:  "pass",
			token:     "token",
		},
	}
	s := d.GetServer()

	pb.RegisterDaemonServer(s, d)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
}

func bufDialer(string, time.Duration) (net.Conn, error) {
	return lis.Dial()
}

func TestNoToken(t *testing.T) {
	ctx := context.Background()
	conn, err := grpc.Dial("bufnet", grpc.WithDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewDaemonClient(conn)
	req := &pb.InviteUserRequest{}
	_, err = client.InviteUser(ctx, req)
	if err == nil {
		t.Fatalf("Expected an error, but did not receive one")
	}
}

func TestInviteUser(t *testing.T) {

	cases := []struct {
		name     string
		keyValue string
		err      error
	}{
		{"OK", "1", nil},
		{"Error in retrieving SignupKey", "", fmt.Errorf("failure")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			d := &daemon{
				uh: testUserHub{
					signupKey: c.keyValue,
					err:       c.err,
				},
			}
			req := &pb.InviteUserRequest{}
			resp, _ := d.InviteUser(ctx, req)

			if resp.Key != c.keyValue {
				t.Fatalf("Expected key '%s', but got '%s'", c.keyValue, resp.Key)
			}
			expectedErrMsg := ""
			if c.err != nil {
				expectedErrMsg = c.err.Error()
			}

			if resp.Error != expectedErrMsg {
				t.Fatalf("Expected error: '%s', but got '%s'", expectedErrMsg, resp.Error)
			}
		})
	}
}

func TestSignupUser(t *testing.T) {
	cases := []struct {
		name     string
		req      pb.SignupUserRequest
		expected pb.LoginUserResponse
	}{
		{
			"OK",
			pb.SignupUserRequest{
				Key:      "keyval",
				Username: "user",
				Password: "pass",
			},
			pb.LoginUserResponse{
				Token: "token",
				Error: "",
			},
		},
		{
			"Invalid signup key",
			pb.SignupUserRequest{
				Key:      "invalid-key",
				Username: "user",
				Password: "pass",
			},
			pb.LoginUserResponse{
				Token: "",
				Error: "failure",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			conn, err := grpc.Dial("bufnet", grpc.WithDialer(bufDialer), grpc.WithInsecure())
			if err != nil {
				t.Fatalf("Failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewDaemonClient(conn)
			resp, _ := client.SignupUser(ctx, &c.req)
			if resp.Error != c.expected.Error {
				t.Fatalf("Expected error '%s', but got '%s'", c.expected.Error, resp.Error)
			}
			if resp.Token != c.expected.Token {
				t.Fatalf("Expected token '%s', but got '%s'", c.expected.Token, resp.Token)
			}
		})
	}
}

func TestLoginUser(t *testing.T) {
	cases := []struct {
		name     string
		req      pb.LoginUserRequest
		expected pb.LoginUserResponse
	}{
		{
			"OK",
			pb.LoginUserRequest{
				Username: "user",
				Password: "pass",
			},
			pb.LoginUserResponse{
				Token: "token",
				Error: "",
			},
		},
		{
			"Invalid credentials",
			pb.LoginUserRequest{
				Username: "user",
				Password: "invalid-password",
			},
			pb.LoginUserResponse{
				Token: "",
				Error: "failure",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			conn, err := grpc.Dial("bufnet", grpc.WithDialer(bufDialer), grpc.WithInsecure())
			if err != nil {
				t.Fatalf("Failed to dial bufnet: %v", err)
			}
			defer conn.Close()

			client := pb.NewDaemonClient(conn)

			resp, _ := client.LoginUser(ctx, &c.req)
			if resp.Error != c.expected.Error {
				t.Fatalf("Expected error '%s', but got '%s'", c.expected.Error, resp.Error)
			}
			if resp.Token != c.expected.Token {
				t.Fatalf("Expected token '%s', but got '%s'", c.expected.Token, resp.Token)
			}
		})
	}
}

type createEventServer struct {
	pb.Daemon_CreateEventServer
}

type testEvent struct {
	started bool
	conf    event.Config
	groups  []event.Group
	event.Event
}

func (ev *testEvent) Start(context.Context) error {
	ev.started = true
	return nil
}

func (ev *testEvent) Connect(*mux.Router) {}

func (ev *testEvent) Close() { ev.started = false }

func (ev *testEvent) GetConfig() event.Config {
	return ev.conf
}

func (ev *testEvent) GetGroups() []event.Group {
	return ev.groups
}

type testEventHost struct {
	ev event.Event
	EventHost
}

func (eh *testEventHost) CreateEvent(event.Config) (event.Event, error) {
	return eh.ev, nil
}

func TestCreateEvent(t *testing.T) {
	ev := &testEvent{started: false}

	d := daemon{
		conf:   &Config{Host: "localhost"},
		mux:    mux.NewRouter(),
		events: make(map[string]event.Event),
		eh:     &testEventHost{ev: ev},
	}
	req := pb.CreateEventRequest{
		Name:      "Event 1",
		Tag:       "ev1",
		Frontends: []string{"frontend1"},
	}

	resp := createEventServer{}
	err := d.CreateEvent(&req, &resp)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectedEvents := 1
	if len(d.events) != expectedEvents {
		t.Fatalf("Expected %d event, got %d", expectedEvents, len(d.events))
	}
	time.Sleep(1 * time.Millisecond) // wait for goroutine to finish
	if !ev.started {
		t.Fatalf("Expected event to be started, but it is not")
	}
}

type stopEventServer struct {
	pb.Daemon_StopEventServer
}

func TestStopEvent(t *testing.T) {
	ev1 := &testEvent{started: true}
	ev2 := &testEvent{started: true}

	d := daemon{
		events: map[string]event.Event{
			"ev1": ev1,
			"ev2": ev2,
		},
	}

	req := pb.StopEventRequest{
		Tag: "ev1",
	}

	resp := stopEventServer{}
	err := d.StopEvent(&req, &resp)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	expectedEvents := 1
	if len(d.events) != expectedEvents {
		t.Fatalf("Expected %d event, got %d", expectedEvents, len(d.events))
	}
	for k := range d.events {
		if k == "ev1" {
			t.Fatalf("Expected ev1 to be removed from daemon, but still exists")
		}
	}
	if ev1.started {
		t.Fatalf("Expected ev1 to be closed, but it is still running")
	}
	if !ev2.started {
		t.Fatalf("Expected ev2 to be running, but it is closed")
	}
}

func TestListEvents(t *testing.T) {
	ctx := context.Background()

	ev := &testEvent{
		conf: event.Config{
			LabConfig: lab.LabConfig{
				Exercises: []exercise.Config{ // define three empty exercises
					{}, {}, {},
				},
			},
		},
		groups: []event.Group{ // define two empty groups
			{}, {},
		},
	}

	d := daemon{
		events: map[string]event.Event{
			"ev": ev,
		},
	}

	req := pb.ListEventsRequest{}

	resp, err := d.ListEvents(ctx, &req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(resp.Events) != 1 {
		t.Fatalf("Expected %d event, got %d", 1, len(resp.Events))
	}
	if resp.Events[0].ExerciseCount != 3 {
		t.Fatalf("Expected %d exercises, got %d", 3, resp.Events[0].ExerciseCount)
	}
	if resp.Events[0].GroupCount != 2 {
		t.Fatalf("Expected %d groups, got %d", 2, resp.Events[0].GroupCount)

	}
}
