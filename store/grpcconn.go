package store

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	eproto "github.com/aau-network-security/haaukins/exercise/ex-proto"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	jwt "github.com/golang-jwt/jwt/v4"
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

type ServiceConfig struct {
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

// NewExerciseClientConn does not require CA file to communicate
// due to the fact that a script is running on Gitlab CI to push
// exercises to the service
func NewExerciseClientConn(config ServiceConfig) (eproto.ExerciseStoreClient, error) {
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
			log.Error().Msgf("[exercise-service]: Error on dialing database service: %v", err)
			return nil, TranslateRPCErr(err)
		}
		client := eproto.NewExerciseStoreClient(conn)
		return client, nil
	}
	authCreds.Insecure = true
	conn, err := grpc.Dial(config.Grpc, grpc.WithInsecure(), grpc.WithPerRPCCredentials(authCreds))
	if err != nil {
		return nil, TranslateRPCErr(err)
	}
	client := eproto.NewExerciseStoreClient(conn)
	return client, nil
}

func NewGRPClientDBConnection(dbConn ServiceConfig) (pbc.StoreClient, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		AUTH_KEY: dbConn.AuthKey,
	})
	tokenString, err := token.SignedString([]byte(dbConn.SignKey))
	if err != nil {
		return nil, TranslateRPCErr(err)
	}

	authCreds := Creds{Token: tokenString}

	if dbConn.Enabled {
		log.Debug().Bool("TLS", dbConn.Enabled).Msg("[store-service]: secure connection enabled for creating secure db client")

		// Load the client certificates from disk
		certificate, err := tls.LoadX509KeyPair(dbConn.CertFile, dbConn.CertKey)
		log.Info().Str("Certfile", dbConn.CertFile).
			Str("CertKey", dbConn.CertKey).Msg("Certs files")
		if err != nil {
			log.Printf("[store-service]: could not load client key pair: %s", err)
		}

		// Create a certificate pool from the certificate authority
		certPool := x509.NewCertPool()
		ca, err := ioutil.ReadFile(dbConn.CAFile)
		if err != nil {
			log.Printf("[store-service]: BCONN could not read ca certificate: %s", err)
		}

		// Append the certificates from the CA
		// This is chain.pem for letsencrypt
		// can be found in same place with existing certificates
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			log.Error().Msg("[store-service]: failed to append ca certs")
		}

		creds := credentials.NewTLS(&tls.Config{
			// no need to RequireAndVerifyClientCert
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    certPool,
			MinVersion:   tls.VersionTLS12, // disable TLS 1.0 and 1.1
			CipherSuites: []uint16{ // only enable secure algorithms for TLS 1.2
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			},
		})

		dialOpts := []grpc.DialOption{
			grpc.WithTransportCredentials(creds),
			grpc.WithPerRPCCredentials(authCreds),
		}

		conn, err := grpc.Dial(dbConn.Grpc, dialOpts...)
		if err != nil {
			log.Error().Msgf("[store-service]: Error on dialing database service: %v", err)
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
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12, // disable TLS 1.0 and 1.1
		CipherSuites: []uint16{ // only enable secure algorithms for TLS 1.2
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
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
