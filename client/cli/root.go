// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"log"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	color "github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	NoTokenErrMsg     = "token contains an invalid number of segments"
	UnauthorizeErrMsg = "unauthorized"
)

var (
	UnreachableDaemonErr = errors.New("Daemon seems to be unreachable")
	UnauthorizedErr      = errors.New("You seem to not be logged in")
	LocalCertificates    = map[string]string{
		"CERT":         "https://gist.githubusercontent.com/mrturkmen06/c53edc50ca777bcece6fca8c21d62ce1/raw/f13ca172eb70c84e0abdfc957cdb563a9a072dcc/localhost.crt",
		"CERT_KEY":     "https://gist.githubusercontent.com/mrturkmen06/2b11591ddda806ce8fa2036693ee347b/raw/2b17c484b520380935102c5268de9911ef2a16eb/localhost.key",
		"CERT_CA_FILE": "https://gist.githubusercontent.com/mrturkmen06/5de6d51cd398be1c3d7df691fc0e4c71/raw/fb557eba6e55ab753b7ac73f32c5e02e820ed802/haaukins-store.com.crt",
	}
)

type Creds struct {
	Token    string
	Insecure bool
}

func Execute() {
	c, err := NewClient()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	var rootCmd = &cobra.Command{Use: "hkn"}
	rootCmd.AddCommand(
		c.CmdVersion(),
		c.CmdUser(),
		c.CmdEvent(),
		c.CmdEvents(),
		c.CmdExercise(),
		c.CmdExercises(),
		c.CmdFrontend(),
		c.CmdFrontends(),
		c.CmdHost(),
		c.CmdTeam(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func versionCheckInterceptor(ctx context.Context, method string, req interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {

	var header metadata.MD
	if err := invoker(ctx, method, req, reply, cc, append(opts, grpc.Header(&header))...); err != nil {
		return err
	}

	var daemonVersion string
	if v := header["daemon-version"]; len(v) > 0 {
		daemonVersion = v[0]
	}

	ok, err := isClientVersionLessThan(daemonVersion)
	if err != nil {
		return fmt.Errorf("Unable to read daemon's version: %s", err)
	}

	if ok {
		return fmt.Errorf("A new version (daemon version: %s) of this client exists, please update", daemonVersion)
	}

	return nil

}

func (c Creds) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"token": string(c.Token),
	}, nil
}

func (c Creds) RequireTransportSecurity() bool {
	return !c.Insecure
}

type Client struct {
	TokenFile string
	Token     string
	conn      *grpc.ClientConn
	rpcClient pb.DaemonClient
}

func NewClient() (*Client, error) {
	usr, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("Unable to find home directory")
	}

	tokenFile := filepath.Join(usr.HomeDir, ".hkn_token")
	c := &Client{
		TokenFile: tokenFile,
	}

	if _, err := os.Stat(tokenFile); err == nil {
		if err := c.LoadToken(); err != nil {
			return nil, err
		}
	}

	host := os.Getenv("HKN_HOST")
	//todo i have change it for testing purpose
	if host == "" {
		host = "grpc.haaukins.com"
	}

	port := os.Getenv("HKN_PORT")
	if port == "" {
		port = "5454"
	}

	authCreds := Creds{Token: c.Token}
	dialOpts := []grpc.DialOption{
		grpc.WithUnaryInterceptor(versionCheckInterceptor),
	}

	ssl_off := os.Getenv("HKN_SSL_OFF")
	endpoint := fmt.Sprintf("%s:%s", host, port)
	var creds credentials.TransportCredentials
	if strings.ToLower(ssl_off) == "true" {
		authCreds.Insecure = true
		dialOpts = append(dialOpts,
			grpc.WithInsecure(),
			grpc.WithPerRPCCredentials(authCreds))
	} else {
		if host == "localhost" {
			devCertPool := x509.NewCertPool()
			creds = setCertConfig(false, devCertPool)
		} else {
			certPool, _ := x509.SystemCertPool()
			creds = setCertConfig(true, certPool)
		}
		dialOpts = append(dialOpts,
			grpc.WithTransportCredentials(creds),
			grpc.WithPerRPCCredentials(authCreds))
	}

	conn, err := grpc.Dial(endpoint, dialOpts...)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	c.rpcClient = pb.NewDaemonClient(conn)

	return c, nil
}

// Downloads necessary localhost certificates
// for local development
func downloadCerts(certMap map[string]string) error {
	_, err := os.Stat("localcerts")
	if os.IsNotExist(err) {
		errDir := os.MkdirAll("localcerts", 0755)
		if errDir != nil {
			log.Fatal(err)
		}
		for k, v := range certMap {
			// Get the data
			resp, err := http.Get(v)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			// Create the file

			out, err := os.Create("localcerts/" + k)
			if err != nil {
				return err
			}

			defer out.Close()
			// Write the body to file
			_, err = io.Copy(out, resp.Body)
			if err != nil {
				return err
			}
		}
	}
	return nil

}

func setCertConfig(isProd bool, certPool *x509.CertPool) credentials.TransportCredentials {
	var certificates []tls.Certificate
	creds := credentials.NewTLS(&tls.Config{
		RootCAs:    certPool,
		MinVersion: tls.VersionTLS12, // disable TLS 1.0 and 1.1
		CipherSuites: []uint16{ // only enable secure algorithms for TLS 1.2
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
	})
	// todo: this will gonna change
	// Create a certificate pool from the certificate authority
	if !isProd {
		if err := downloadCerts(LocalCertificates); err != nil {
			log.Printf("Error on dowloading certificates from given path %s", err)
		}
		certificate, err := tls.LoadX509KeyPair("localcerts/CERT", "localcerts/CERT_KEY")
		if err != nil {
			log.Printf("could not load client key pair: %s", err)
		}
		ca, err := ioutil.ReadFile("localcerts/CERT_CA_FILE")
		if err != nil {
			log.Printf("could not read ca certificate: %s", err)
		}
		// Append the certificates from the CA
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			log.Println("failed to append ca certs")
		}
		certificates = append(certificates, certificate)
		creds = credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{certificate},
			RootCAs:      certPool,
			MinVersion:   tls.VersionTLS12, // disable TLS 1.0 and 1.1
			CipherSuites: []uint16{ // only enable secure algorithms for TLS 1.2
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			},
		})
	}
	return creds
}

func (c *Client) LoadToken() error {
	raw, err := ioutil.ReadFile(c.TokenFile)
	if err != nil {
		return err
	}

	c.Token = string(raw)
	return nil
}

func (c *Client) SaveToken() error {
	return ioutil.WriteFile(c.TokenFile, []byte(c.Token), 0644)
}

func (c *Client) Close() {
	c.conn.Close()
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
			return UnreachableDaemonErr
		}

		return err
	}

	return err
}

func PrintError(err error) {
	err = TranslateRPCErr(err)
	fmt.Printf("%s %s\n", color.Red("<!>"), color.Red(err.Error()))
}

func PrintWarning(s string) {
	fmt.Printf("%s %s\n", color.Brown("<?>"), color.Brown(s))
}

func ReadSecret(inputHint string) (string, error) {
	fmt.Printf(inputHint)
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Printf("\n")
	if err != nil {
		return "", err
	}

	return string(bytePassword), nil
}
