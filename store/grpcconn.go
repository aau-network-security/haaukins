package store

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"

	eservice "github.com/aau-network-security/haaukins/store/eproto"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/dgrijalva/jwt-go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

const (
	NoTokenErrMsg     = "token contains an invalid number of segments"
	UnauthorizeErrMsg = "unauthorized"
	AUTH_KEY          = "au"
)

var (
	UnreachableDBErr = errors.New("Database seems to be unreachable")
	UnauthorizedErr  = errors.New("You seem to not be logged in")
)

type Config struct {
	Grpc     string
	AuthKey  string
	SignKey  string
	Enabled  bool
	CertFile string
	CertKey  string
	CAFile   string
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

func NewExerciseClientConn(config Config) (eservice.ExerciseStoreClient, error) {
	creds := enableClientCertificates()
	authCreds, err := constructAuthCreds(config.AuthKey, config.SignKey)
	if err != nil {
		return nil, fmt.Errorf("[exercise-service]: Error in constructing auth credentials %v", err)
	}
	if config.Enabled {
		log.Debug().Bool("TLS", config.Enabled).Msg(" secure connection enabled for creating secure [exercise-service] client")
		dialOpts := []grpc.DialOption{
			grpc.WithTransportCredentials(creds),
			grpc.WithPerRPCCredentials(authCreds),
		}
		conn, err := grpc.Dial(config.Grpc, dialOpts...)
		if err != nil {
			log.Error().Msgf("Error on dialing database service: %v", err)
			return nil, TranslateRPCErr(err)
		}
		client := eservice.NewExerciseStoreClient(conn)
		return client, nil
	}
	authCreds.Insecure = true
	conn, err := grpc.Dial(config.Grpc, grpc.WithInsecure(), grpc.WithPerRPCCredentials(authCreds))
	if err != nil {
		return nil, TranslateRPCErr(err)
	}
	client := eservice.NewExerciseStoreClient(conn)
	return client, nil
}

func NewGRPClientDBConnection(dbConn Config) (pbc.StoreClient, error) {
	creds := enableClientCertificates()
	authCreds, err := constructAuthCreds(dbConn.AuthKey, dbConn.SignKey)
	if err != nil {
		return nil, fmt.Errorf("[store-service]: Error in constructing auth credentials %v", err)
	}
	if dbConn.Enabled {
		log.Debug().Bool("TLS", dbConn.Enabled).Msg(" secure connection enabled for creating secure db client")
		dialOpts := []grpc.DialOption{
			grpc.WithTransportCredentials(creds),
			grpc.WithPerRPCCredentials(authCreds),
		}
		conn, err := grpc.Dial(dbConn.Grpc, dialOpts...)
		if err != nil {
			log.Error().Msgf("Error on dialing database service: %v", err)
			return nil, TranslateRPCErr(err)
		}
		c := pbc.NewStoreClient(conn)
		return c, nil
	}

	authCreds.Insecure = true
	conn, err := grpc.Dial(dbConn.Grpc, grpc.WithInsecure(), grpc.WithPerRPCCredentials(authCreds))
	if err != nil {
		return nil, TranslateRPCErr(err)
	}
	c := pbc.NewStoreClient(conn)
	return c, nil
}

func constructAuthCreds(authKey, signKey string) (Creds, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		AUTH_KEY: authKey,
	})
	tokenString, err := token.SignedString([]byte(signKey))
	if err != nil {
		return Creds{}, TranslateRPCErr(err)
	}
	authCreds := Creds{Token: tokenString}
	return authCreds, nil
}

func enableClientCertificates() credentials.TransportCredentials {
	// Load the client certificates from disk
	pool, _ := x509.SystemCertPool()
	creds := credentials.NewClientTLSFromCert(pool, "")

	creds = credentials.NewTLS(&tls.Config{
		RootCAs: pool,
	})

	return creds
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
