package daemon

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/aau-network-security/haaukins/client/cli"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

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
				g := store.NewTeam(fmt.Sprintf("team-%d@team.dk", i), "whatever", "",
					fmt.Sprintf("team-%d", i), "", "", time.Now().UTC(), map[string][]string{}, map[string][]string{"sql": []string{"sql"}}, nil)
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
