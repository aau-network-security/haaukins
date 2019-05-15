// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
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
	if host == "" {
		host = "cli.sec-aau.dk"
	}

	port := os.Getenv("HKN_PORT")
	if port == "" {
		port = "5454"
	}

	authCreds := Creds{Token: c.Token}
	dialOpts := []grpc.DialOption{
		grpc.WithUnaryInterceptor(versionCheckInterceptor),
	}

	ssl := os.Getenv("HKN_SSL_OFF")
	if strings.ToLower(ssl) == "true" {
		authCreds.Insecure = true
		dialOpts = append(dialOpts,
			grpc.WithInsecure(),
			grpc.WithPerRPCCredentials(authCreds))
	} else {
		pool, _ := x509.SystemCertPool()
		creds := credentials.NewClientTLSFromCert(pool, "")
		dialOpts = append(dialOpts,
			grpc.WithTransportCredentials(creds),
			grpc.WithPerRPCCredentials(authCreds))
	}

	endpoint := fmt.Sprintf("%s:%s", host, port)
	conn, err := grpc.Dial(endpoint, dialOpts...)
	if err != nil {
		return nil, err
	}
	c.conn = conn
	c.rpcClient = pb.NewDaemonClient(conn)

	return c, nil
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
