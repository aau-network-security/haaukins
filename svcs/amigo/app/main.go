package main

import (

	"context"
	"fmt"
	"github.com/aau-network-security/haaukins/store"
	pb "github.com/aau-network-security/haaukins/store/proto"
	"github.com/aau-network-security/haaukins/svcs/amigo"
	mockserver "github.com/aau-network-security/haaukins/testing"
	"google.golang.org/grpc"
	"log"
	"net/http"
)

func main() {

	dialer, close := mockserver.Create()
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
		{
			Tag:         "test1",
			Name:        "test1",
			EnvVar:      "",
			Static:      "",
			Points:      0,
			Description: "this is a test",
			Category:    "Forensics",
		},
		{
			Tag:         "test-11",
			Name:        "test-11",
			EnvVar:      "",
			Static:      "",
			Points:      10,
			Description: "this is a test",
			Category:    "Forensics",
		},
		{
			Tag:         "test-31",
			Name:        "test-31",
			EnvVar:      "",
			Static:      "",
			Points:      5,
			Description: "this is a test",
			Category:    "Forensics",
		},
		{
			Tag:         "test-21",
			Name:        "test-21",
			EnvVar:      "",
			Static:      "",
			Points:      5,
			Description: "this is a test",
			Category:    "Web exploitation",
		},
		{
			Tag:         "test-131",
			Name:        "test-131",
			EnvVar:      "",
			Static:      "",
			Points:      3,
			Description: "this is a test",
			Category:    "Web exploitation",
		},
		{
			Tag:         "test-231",
			Name:        "test-231",
			EnvVar:      "",
			Static:      "",
			Points:      4,
			Description: "this is a test",
			Category:    "Web exploitation",
		},
		{
			Tag:         "test2",
			Name:        "test2",
			EnvVar:      "",
			Static:      "",
			Points:      0,
			Description: "this is a test",
			Category:    "Web exploitation",
		},
		{
			Tag:         "test-21",
			Name:        "test-12",
			EnvVar:      "",
			Static:      "",
			Points:      10,
			Description: "this is a test",
			Category:    "Web exploitation",
		},
		{
			Tag:         "test-23",
			Name:        "test-23",
			EnvVar:      "",
			Static:      "",
			Points:      5,
			Description: "this is a test",
			Category:    "Web exploitation",
		},
		{
			Tag:         "test-22",
			Name:        "test-22",
			EnvVar:      "",
			Static:      "",
			Points:      5,
			Description: "this is a test",
			Category:    "Web exploitation",
		},
		{
			Tag:         "test-123",
			Name:        "test-123",
			EnvVar:      "",
			Static:      "",
			Points:      3,
			Description: "this is a test",
			Category:    "Web exploitation",
		},
		{
			Tag:         "test-223",
			Name:        "test-223",
			EnvVar:      "",
			Static:      "",
			Points:      4,
			Description: "this is a test",
			Category:    "Web exploitation",
		},
		{
			Tag:         "test3",
			Name:        "test3",
			EnvVar:      "",
			Static:      "",
			Points:      0,
			Description: "this is a test",
			Category:    "Forensics",
		},
		{
			Tag:         "test-31",
			Name:        "test-31",
			EnvVar:      "",
			Static:      "",
			Points:      10,
			Description: "this is a test",
			Category:    "Forensics",
		},
		{
			Tag:         "test-33",
			Name:        "test-33",
			EnvVar:      "",
			Static:      "",
			Points:      5,
			Description: "this is a test",
			Category:    "Forensics",
		},
		{
			Tag:         "test-32",
			Name:        "test-32",
			EnvVar:      "",
			Static:      "",
			Points:      5,
			Description: "this is a test",
			Category:    "Web exploitation",
		},
		{
			Tag:         "test-133",
			Name:        "test-133",
			EnvVar:      "",
			Static:      "",
			Points:      3,
			Description: "this is a test",
			Category:    "Cryptography",
		},
		{
			Tag:         "test-233",
			Name:        "test-233",
			EnvVar:      "",
			Static:      "",
			Points:      4,
			Description: "this is a test",
			Category:    "Binary",
		},
	}
	am := amigo.NewAmigo(ts, chals)

	log.Fatal(http.ListenAndServe(":8080", am.Handler(nil, http.NewServeMux())))
}
