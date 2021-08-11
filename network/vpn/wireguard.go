package wg

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"strconv"
	"strings"

	"google.golang.org/grpc/status"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	UnreachableVPNServiceErr = errors.New("Wireguard service is not running !")
	UnauthorizedErr          = errors.New("Unauthorized attempt to use VPN service ")
	NoTokenErrMsg            = "token contains an invalid number of segments"
	UnauthorizeErrMsg        = "unauthorized"
	AUTH_KEY                 = "wg"
)

type WireGuardConfig struct {
	Endpoint string
	Port     uint64
	AuthKey  string
	SignKey  string
	Enabled  bool
	CertFile string
	CertKey  string
	CAFile   string
	Dir      string // client configuration file will reside
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

func NewGRPCVPNClient(wgConn WireGuardConfig) (WireguardClient, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		AUTH_KEY: wgConn.AuthKey,
	})
	tokenString, err := token.SignedString([]byte(wgConn.SignKey))
	if err != nil {
		return nil, TranslateRPCErr(err)
	}

	authCreds := Creds{Token: tokenString}

	if wgConn.Enabled {
		log.Debug().Bool("TLS", wgConn.Enabled).Msg(" secure connection enabled for creating secure db client")

		// Load the client certificates from disk
		certificate, err := tls.LoadX509KeyPair(wgConn.CertFile, wgConn.CertKey)
		log.Info().Str("Certfile", wgConn.CertFile).
			Str("CertKey", wgConn.CertKey).Msg("Certs files")
		if err != nil {
			log.Printf("could not load client key pair: %s", err)
		}

		// Create a certificate pool from the certificate authority
		certPool := x509.NewCertPool()
		ca, err := ioutil.ReadFile(wgConn.CAFile)
		if err != nil {
			log.Printf("VPNCONN could not read ca certificate: %s", err)
		}

		// Append the certificates from the CA
		// This is chain.pem for letsencrypt
		// can be found in same place with existing certificates
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			log.Error().Msg("failed to append ca certs")
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

		conn, err := grpc.Dial(wgConn.Endpoint+":"+strconv.FormatUint(wgConn.Port, 10), dialOpts...)
		if err != nil {
			log.Error().Msgf("Error on dialing vpn service: %v", err)
			return nil, TranslateRPCErr(err)
		}
		c := NewWireguardClient(conn)
		return c, nil
	}

	authCreds.Insecure = true
	conn, err := grpc.Dial(wgConn.Endpoint+":"+strconv.FormatUint(wgConn.Port, 10), grpc.WithInsecure(), grpc.WithPerRPCCredentials(authCreds))
	if err != nil {
		return nil, TranslateRPCErr(err)
	}
	c := NewWireguardClient(conn)
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

			return UnreachableVPNServiceErr
		}

		return err
	}

	return err
}
