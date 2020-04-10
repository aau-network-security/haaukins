package store

import (
	"errors"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"strings"
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

	if tls {
		creds, _ := credentials.NewClientTLSFromFile(certFile, "")
		conn, err := grpc.Dial(server, grpc.WithTransportCredentials(creds))
		if err != nil{
			return nil, TranslateRPCErr(err)
		}
		c := pbc.NewStoreClient(conn)
		return c, nil
	}

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