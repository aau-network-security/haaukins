// Copyright (c) 2018-2019 Aalborg University

// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package guacamole

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/aau-network-security/haaukins/store"
	pb "github.com/aau-network-security/haaukins/store/proto"
	mockserver "github.com/aau-network-security/haaukins/testing"
	"google.golang.org/grpc"

	"github.com/aau-network-security/haaukins/lab"
)

const (
	STARTED = 1
	CLOSED  = 2
)

type testGuac struct {
	status int
	Guacamole
}

func (guac *testGuac) Start(ctx context.Context) error {
	guac.status = STARTED
	return nil
}

func (guac *testGuac) Close() error {
	guac.status = CLOSED
	return nil
}

type testLabHub struct {
	status int
	lab    lab.Lab
	err    error
	lab.Hub
}

func (hub *testLabHub) Close() error {
	hub.status = CLOSED
	return nil
}

func TestEvent_StartAndClose(t *testing.T) {
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	dialer, close := mockserver.Create()
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
		}, tmp, client)

		ev := event{
			guac:    &guac,
			labhub:  &hub,
			closers: []io.Closer{&guac, &hub},
			store:   ts,
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
