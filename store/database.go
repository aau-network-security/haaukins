package store

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/dgrijalva/jwt-go"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"io/ioutil"
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

type DBConfig struct {
	Grpc 	string
	AuthKey string
	SignKey string
	Enabled   bool
	CertFile string
	CertKey string
	CAFile	string
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

func NewGRPClientDBConnection(dbConn DBConfig) (pbc.StoreClient, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		AUTH_KEY: dbConn.AuthKey,
	})
	tokenString, err := token.SignedString([]byte(dbConn.SignKey))
	if err != nil {
		return nil, TranslateRPCErr(err)
	}

	authCreds := Creds{Token: tokenString}

	if dbConn.Enabled {
		log.Debug().Bool("TLS",dbConn.Enabled).Msg(" secure connection enabled for creating secure db client")

		// Load the client certificates from disk
		certificate, err := tls.LoadX509KeyPair(dbConn.CertFile, dbConn.CertKey)
		log.Info().Str("Certfile", dbConn.CertFile).
			       Str("CertKey",dbConn.CertKey).Msg("Certs files")
		if err != nil {
			log.Printf("could not load client key pair: %s", err)
		}

		// Create a certificate pool from the certificate authority
		certPool := x509.NewCertPool()
		ca, err := ioutil.ReadFile(dbConn.CAFile)
		if err != nil {
			log.Printf("DBCONN could not read ca certificate: %s", err)
		}

		// Append the certificates from the CA
		// This is chain.pem for letsencrypt
		// can be found in same place with existing certificates
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			log.Error().Msg("failed to append ca certs")
		}

		creds := credentials.NewTLS(&tls.Config{
			ServerName:   dbConn.Grpc,
			Certificates: []tls.Certificate{certificate},
			RootCAs:      certPool,
		})

		dialOpts := []grpc.DialOption{
			grpc.WithTransportCredentials(creds),
			grpc.WithPerRPCCredentials(authCreds),
		}

		conn, err := grpc.Dial(dbConn.Grpc, dialOpts...)
		if err != nil{
			log.Error().Msgf("Error on dialing database service: %v",err)
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