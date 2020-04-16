package main

import (

	"context"
	"github.com/aau-network-security/haaukins/store"
	pb "github.com/aau-network-security/haaukins/store/proto"
	"github.com/aau-network-security/haaukins/svcs/amigo"
	"google.golang.org/grpc"
	"log"
	"net/http"
)

func main() {

	dialer, close := store.CreateTestServer()
	defer close()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithDialer(dialer),
		grpc.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewStoreClient(conn)
	ts, _ := store.NewEventStore(store.EventConfig{
		Name:           "Test Event",
		Tag:            "test",
		Available:      1,
		Capacity:       2,
		Lab:            store.Lab{},
		StartedAt:      nil,
		FinishExpected: nil,
		FinishedAt:     nil,
	}, "events", client)

	chals := []store.FlagConfig{
		{
			Tag:         "test",
			Name:        "test",
			EnvVar:      "",
			Static:      "",
			Points:      0,
			Description: "this is a test",
			Category:    "Test",
		},
	}
	am := amigo.NewAmigo(ts, chals)

	log.Fatal(http.ListenAndServe(":8080", am.Handler(nil, http.NewServeMux())))
}
