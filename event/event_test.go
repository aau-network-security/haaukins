// Copyright (c) 2018-2019 Aalborg University

// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package event

import (
	"context"
	pb "github.com/aau-network-security/haaukins/store/proto"
	"google.golang.org/grpc"
	"io"
	"testing"

	"github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/guacamole"
)

const (
	STARTED = 1
	CLOSED  = 2
)

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

type testLabHub struct {
	status int
	lab    lab.Lab
	err    error
	lab.Hub
}

func (hub *testLabHub) Queue() <-chan lab.Lab {
	return nil
}

func (hub *testLabHub) Close() error {
	hub.status = CLOSED
	return nil
}

func (hub *testLabHub) GetLabByTag(string) (lab.Lab, error) {
	return nil, nil
}

func TestEvent_StartAndClose(t *testing.T) {
	dialer, close := store.CreateTestServer()
	defer close()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithDialer(dialer),
		grpc.WithInsecure(),
	)
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewStoreClient(conn)

	t.Run("Normal", func(t *testing.T) {
		guac := testGuac{}
		hub := testLabHub{}

		ts, _ := store.NewEventStore(store.EventConfig{
			Name:           "Test Event",
			Tag:            "test",
			Available:      1,
			Capacity:       2,
			Lab:            store.Lab{},
			StartedAt:      nil,
			FinishExpected: nil,
			FinishedAt:     nil,
		}, client)

		ev := event{
			guac:    &guac,
			labhub:  &hub,
			closers: []io.Closer{&guac, &hub},
			store:  ts,
		}

		ev.Start(context.Background())

		if guac.status != STARTED {
			t.Fatalf("Expected Guacamole to be started, but hasn't")
		}

		ev.Close()

		if guac.status != CLOSED {
			t.Fatalf("Expected Guacamole to be stopped, but hasn't")
		}
		if hub.status != CLOSED {
			t.Fatalf("Expected LabHub to be stopped, but hasn't")
		}
	})

}
