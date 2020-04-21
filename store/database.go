package store

import (
	"context"
	"errors"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/dgrijalva/jwt-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"strings"
)

const (
	NoTokenErrMsg     = "token contains an invalid number of segments"
	UnauthorizeErrMsg = "unauthorized"
	AUTH_KEY    = "au"
)

var (
	UnreachableDBErr 	= errors.New("Database seems to be unreachable")
	UnauthorizedErr     = errors.New("You seem to not be logged in")
)

type DBConn struct {
	Server 		string
	CertFile    string
	Tls			bool
	AuthKey     string
	SignKey     string
}

type Creds struct {
	Token    string
	Insecure bool
}

func (c Creds) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"token": string(c.Token),
	}, nil
}

func (c Creds) RequireTransportSecurity() bool {
	return !c.Insecure
}

func NewGRPClientDBConnection(dbConn DBConn) (pbc.StoreClient, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		AUTH_KEY: dbConn.AuthKey,
	})
	tokenString, err := token.SignedString([]byte(dbConn.SignKey))
	if err != nil {
		return nil, TranslateRPCErr(err)
	}

	authCreds := Creds{Token: tokenString}

	if dbConn.Tls {

		creds, _ := credentials.NewClientTLSFromFile(dbConn.CertFile, "")

		dialOpts := []grpc.DialOption{
			grpc.WithTransportCredentials(creds),
			grpc.WithPerRPCCredentials(authCreds),
		}
		conn, err := grpc.Dial(dbConn.Server, dialOpts...)
		if err != nil{
			return nil, TranslateRPCErr(err)
		}
		c := pbc.NewStoreClient(conn)
		return c, nil
	}

	authCreds.Insecure = true
	conn, err := grpc.Dial(dbConn.Server, grpc.WithInsecure(), grpc.WithPerRPCCredentials(authCreds))
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