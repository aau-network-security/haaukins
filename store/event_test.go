// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package store_test

import (
	"context"
	"github.com/aau-network-security/haaukins/store"
	pb "github.com/aau-network-security/haaukins/store/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"testing"
	"time"
)

const (
	address     = "localhost:50051"
)

func TestNewTeam(t *testing.T) {
	password := "some_password"
	team := store.NewTeam("some@email.com", "some name", password, "", "", "",nil)

	if team.GetHashedPassword() == password {
		t.Fatalf("expected password to be hashed")
	}
}

func TestTeamSolveTask(t *testing.T) {

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
	_, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	team := store.NewTeam("some@email.com", "some name", "password", "", "", "", client)

	chal := store.Challenge{
		Name:  "FTP",
		Tag:   "ftp",
		Value: store.NewFlag().String(),
	}

	flag, _ := team.AddChallenge(chal)
	if err := team.VerifyFlag(chal, flag); err != nil {
		t.Fatalf("expected no error when solving task for team: %s", err)
	}

	if err := team.VerifyFlag(chal, flag); err == nil {
		t.Fatalf("expected error when solving challenge already solved: %s", err)
	}

	if err := team.VerifyFlag(store.Challenge{
		Name:  "Test",
		Tag:   "test",
		Value: store.NewFlag().String(),
	}, flag); err == nil {
		t.Fatalf("expected error when solving unknown challenge")
	}

}

func TestCreateToken(t *testing.T) {

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

	team := store.NewTeam("some@email.com", "some name", "password", "", "", "", client)

	tt := []struct {
		name  string
		team  *store.Team
		token string
		err   string
	}{
		{name: "Normal", team: team, token: uuid.New().String()},
		{name: "Empty token", team: team, token: "", err: "Token cannot be empty"},
		{name: "Unknown team", token: uuid.New().String(), err: "Unknown team"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
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

			var team store.Team
			if tc.team != nil {
				team = *tc.team
				if err := ts.SaveTeam(&team); err != nil {
					t.Fatalf("expected no error when creating team")
				}
			}

			err = ts.SaveTokenForTeam(tc.token, &team)
			if err != nil {
				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: \"%s\") when creating token: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("received error when creating token, but expected none: %s", err)
			}

			if tc.err != "" {
				t.Fatalf("expected error but received none: %s", tc.err)
			}

		})
	}
}

func TestGetTokenForTeam(t *testing.T) {
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

	team := store.NewTeam("some@email.com", "some name", "password", "", "", "", client)

	if err := ts.SaveTeam(team); err != nil {
		t.Fatalf("expected no error when creating team")
	}

	token := "token-to-test"
	err = ts.SaveTokenForTeam(token, team)
	if err != nil {
		t.Fatalf("expected no error when creating token")
	}

	tt := []struct {
		name  string
		team  store.Team
		token string
		err   string
	}{
		{name: "Normal", token: token, team: *team},
		{name: "No team", token: "invalid-token", err: "Unknown token"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			team, err := ts.GetTeamByToken(tc.token)
			if err != nil {
				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: \"%s\") when getting team by token: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("received error when getting team by token, but expected none: %s", err)
			}

			if tc.err != "" {
				t.Fatalf("expected error but received none: %s", tc.err)
			}

			if team.Email() != tc.team.Email() {
				t.Fatalf("received unexpected team: %+v", team)
			}
		})
	}
}

func TestDeleteToken(t *testing.T) {
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

	tt := []struct {
		name        string
		inputToken  string
		deleteToken string
		err         string
	}{
		{name: "Normal", inputToken: "some_token", deleteToken: "some_token"},
		{name: "Empty token", inputToken: "some_token", deleteToken: ""},
		{name: "Unknown token", inputToken: "some_token", deleteToken: "some_other_token", err: "Unknown token"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

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

			team := store.NewTeam("some@email.com", "some name", "password", "", "", "", client)

			if err := ts.SaveTeam(team); err != nil {
				t.Fatalf("expected no error when creating team")
			}

			err := ts.SaveTokenForTeam(tc.inputToken, team)
			if err != nil {
				t.Fatalf("expected no error when creating token")
			}

			err = ts.DeleteToken(tc.deleteToken)
			if err != nil {
				if tc.err != "" {
					if tc.err != err.Error() {
						t.Fatalf("unexpected error (expected: \"%s\") when getting team by token: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("received error when getting team by token, but expected none: %s", err)
			}
		})
	}
}
