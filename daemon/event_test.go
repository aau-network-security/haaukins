package daemon

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/aau-network-security/haaukins/client/cli"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func TestCreateEvent(t *testing.T) {
	t.Skip("Due to GetEventStatus function in CreateEvent, it throws error")
	t.Skipped()
	// mock store database should be initialized
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
				ehost: fakeEventHost{
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
	t.Skip("Due to database client function in StopEvent, it throws error")
	t.Skipped()
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

			d.startEvent(&ev)

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
	t.Skip("Due to database client function in ListEvents, it throws error")
	t.Skipped()
	tt := []struct {
		name         string
		unauthorized bool
		count        int
		err          string
		startedTime  string
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
			startedAt, _ := time.Parse(tc.startedTime, displayTimeFormat)
			finishDate, _ := time.Parse(time.Now().Format(displayTimeFormat), displayTimeFormat)
			for i := 1; i <= tc.count; i++ {
				tempEvent := *ev
				tempEvent.conf = store.EventConfig{StartedAt: &startedAt, Tag: store.Tag(fmt.Sprintf("tst-%d", i)), FinishExpected: &finishDate}
				d.startEvent(&tempEvent)
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

func TestRemoveDuplicates(t *testing.T) {
	tests := []struct {
		name      string
		exercises []string
		want      []string
	}{
		{name: "List with duplicated values", exercises: []string{"test1", "test", "test2", "test1", "test3", "test23"}, want: []string{"test1", "test", "test2", "test3", "test23"}},
		{name: "List without duplicated values", exercises: []string{"test", "test1", "test2"}, want: []string{"test", "test1", "test2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeDuplicates(tt.exercises); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("removeDuplicates() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckTime(t *testing.T) {
	now := time.Now()
	noDelay := now.Add(time.Hour * 24).Format(displayTimeFormat)
	tests := []struct {
		name       string
		customTime string
		want       bool
	}{
		{name: "Current Time", customTime: time.Now().Format(displayTimeFormat), want: true},
		{name: "Delayed", customTime: displayTimeFormat, want: true},
		{name: "NotDelayed", customTime: noDelay, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDelayed(tt.customTime); got != tt.want {
				t.Errorf("isDelayed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func toTime(year, month, day, hour, minute, second int) time.Time {
	return time.Date(year, time.Month(month), day, hour, minute, second, 0000, time.UTC)
}
