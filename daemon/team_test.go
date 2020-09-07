package daemon

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/aau-network-security/haaukins/store"

	"github.com/aau-network-security/haaukins/client/cli"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/virtual"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

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
