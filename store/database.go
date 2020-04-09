package store

import (
	"context"
	"errors"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"strings"
	"time"
)

const (
	NoTokenErrMsg     = "token contains an invalid number of segments"
	UnauthorizeErrMsg = "unauthorized"
)

var (
	UnreachableDBErr 	= errors.New("Database seems to be unreachable")
	UnauthorizedErr     = errors.New("You seem to not be logged in")
)

func NewGRPClientDBConnection(server, certFile string, tls bool) (pbc.StoreClient, error) {

	//if tls {
	//	creds, _ := credentials.NewClientTLSFromFile(certFile, "")
	//	conn, err := grpc.Dial(server, grpc.WithTransportCredentials(creds))
	//	if err != nil{
	//		return nil, TranslateRPCErr(err)
	//	}
	//	c := pbc.NewStoreClient(conn)
	//	return c, nil
	//}

	conn, err := grpc.Dial(server, grpc.WithInsecure())
	if err != nil {
		return nil, TranslateRPCErr(err)
	}
	c := pbc.NewStoreClient(conn)
	return c, nil
}

func TranslateRPCErr(err error) error {
	st, ok := status.FromError(err)
	if ok {
		msg := st.Message()
		switch {
		case UnauthorizeErrMsg == msg:
			return UnauthorizedErr

		case NoTokenErrMsg == msg:
			return UnauthorizedErr

		case strings.Contains(msg, "TransientFailure"):
			return UnreachableDBErr
		}

		return err
	}

	return err
}

// FROM HERE ITS FOR TESTING PURPOSE

type serverTest struct {
	pbc.UnimplementedStoreServer
}

func (s serverTest) AddEvent(context.Context, *pbc.AddEventRequest) (*pbc.InsertResponse, error) {
	return &pbc.InsertResponse{Message:"ok"}, nil
}

func (s serverTest) AddTeam(context.Context, *pbc.AddTeamRequest) (*pbc.InsertResponse, error) {
	return &pbc.InsertResponse{Message:"ok"}, nil
}

func (s serverTest) GetEvents(context.Context, *pbc.EmptyRequest) (*pbc.GetEventResponse, error) {
	return &pbc.GetEventResponse{}, nil
}

func (s serverTest) GetEventTeams(context.Context, *pbc.GetEventTeamsRequest) (*pbc.GetEventTeamsResponse, error) {
	return &pbc.GetEventTeamsResponse{}, nil
}

func (s serverTest) UpdateEventFinishDate(context.Context, *pbc.UpdateEventRequest) (*pbc.UpdateResponse, error) {
	return &pbc.UpdateResponse{Message:"ok"}, nil
}

func (s serverTest) UpdateTeamSolvedChallenge(context.Context, *pbc.UpdateTeamSolvedChallengeRequest) (*pbc.UpdateResponse, error) {
	return &pbc.UpdateResponse{Message:"ok"}, nil
}

func (s serverTest) UpdateTeamLastAccess(context.Context, *pbc.UpdateTeamLastAccessRequest) (*pbc.UpdateResponse, error) {
	return &pbc.UpdateResponse{Message:"ok"}, nil
}

func CreateTestServer() (func(string, time.Duration) (net.Conn, error), func() error){
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