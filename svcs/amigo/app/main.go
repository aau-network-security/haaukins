package main

import (

	"context"
	"fmt"
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


	team := store.NewTeam("some@email.com", "some name", "password", "", "", "", client)

	if err := ts.SaveTeam(team); err != nil {
		fmt.Print("expected no error when creating team")
	}

	team2 := store.NewTeam("some2@email.com", "some name", "password", "", "", "", client)

	if err := ts.SaveTeam(team2); err != nil {
		fmt.Print("expected no error when creating team")
	}

	chals := []store.FlagConfig{
		{
			Tag:         "test",
			Name:        "test",
			EnvVar:      "",
			Static:      "",
			Points:      0,
			Description: "this is a test",
			Category:    "Forensics",
		},
		{
			Tag:         "test-1",
			Name:        "test-1",
			EnvVar:      "",
			Static:      "",
			Points:      10,
			Description: "this is a test",
			Category:    "Forensics",
		},
		{
			Tag:         "test-3",
			Name:        "test-3",
			EnvVar:      "",
			Static:      "",
			Points:      5,
			Description: "this is a test",
			Category:    "Forensics",
		},
		{
			Tag:         "test-2",
			Name:        "test-2",
			EnvVar:      "",
			Static:      "",
			Points:      5,
			Description: "this is a test",
			Category:    "Web exploitation",
		},
		{
			Tag:         "test-13",
			Name:        "test-13",
			EnvVar:      "",
			Static:      "",
			Points:      3,
			Description: "this is a test",
			Category:    "Reverse Engineering",
		},
		{
			Tag:         "test-23",
			Name:        "test-23",
			EnvVar:      "",
			Static:      "",
			Points:      4,
			Description: "this is a test",
			Category:    "Reverse Engineering",
		},
	}
	am := amigo.NewAmigo(ts, chals)

	log.Fatal(http.ListenAndServe(":8080", am.Handler(nil, http.NewServeMux())))
}
