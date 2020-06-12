package testing

import (
	"context"
	"net"
	"time"

	pbc "github.com/aau-network-security/haaukins/store/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

// FROM HERE ITS FOR TESTING PURPOSE

type serverTest struct {
	pbc.UnimplementedStoreServer
}

func (s serverTest) AddEvent(context.Context, *pbc.AddEventRequest) (*pbc.InsertResponse, error) {
	return &pbc.InsertResponse{Message: "ok"}, nil
}

func (s serverTest) AddTeam(context.Context, *pbc.AddTeamRequest) (*pbc.InsertResponse, error) {
	return &pbc.InsertResponse{Message: "ok"}, nil
}

func (s serverTest) GetEvents(context.Context, *pbc.GetEventRequest) (*pbc.GetEventResponse, error) {
	return &pbc.GetEventResponse{}, nil
}

func (s serverTest) GetEventTeams(context.Context, *pbc.GetEventTeamsRequest) (*pbc.GetEventTeamsResponse, error) {
	return &pbc.GetEventTeamsResponse{}, nil
}

func (s serverTest) UpdateEventFinishDate(context.Context, *pbc.UpdateEventRequest) (*pbc.UpdateResponse, error) {
	return &pbc.UpdateResponse{Message: "ok"}, nil
}

func (s serverTest) UpdateTeamSolvedChallenge(context.Context, *pbc.UpdateTeamSolvedChallengeRequest) (*pbc.UpdateResponse, error) {
	return &pbc.UpdateResponse{Message: "ok"}, nil
}

func (s serverTest) UpdateTeamLastAccess(context.Context, *pbc.UpdateTeamLastAccessRequest) (*pbc.UpdateResponse, error) {
	return &pbc.UpdateResponse{Message: "ok"}, nil
}

func Create() (func(string, time.Duration) (net.Conn, error), func() error) {
	const oneMegaByte = 1024 * 1024
	lis := bufconn.Listen(oneMegaByte)

	s := grpc.NewServer()
	pbc.RegisterStoreServer(s, &serverTest{})
	go func() {
		s.Serve(lis)
	}()
	dialer := func(string, time.Duration) (net.Conn, error) {
		return lis.Dial()
	}

	return dialer, lis.Close
}
