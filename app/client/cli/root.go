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
	"time"

	"log"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	color "github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

	tokenFile := filepath.Join(usr.HomeDir, ".ntp_token")
	c := &Client{
		TokenFile: tokenFile,
	}

	if _, err := os.Stat(tokenFile); err == nil {
		if err := c.LoadToken(); err != nil {
			return nil, err
		}
	}

	host := os.Getenv("NTP_HOST")
	if host == "" {
		host = "cli.sec-aau.dk"
	}

	port := os.Getenv("NTP_PORT")
	if port == "" {
		port = "5454"
	}

	authCreds := Creds{Token: c.Token}
	dialOpts := []grpc.DialOption{}

	ssl := os.Getenv("NTP_SSL_OFF")
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

func (c *Client) CheckVersionSync() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.rpcClient.Version(ctx, &pb.Empty{})
	if err != nil {
		err = TranslateRPCErr(err)
		switch err {
		case context.DeadlineExceeded:
			return UnreachableDaemonErr
		case UnauthorizedErr:
			return nil
		default:
			return err
		}
	}

	ok, err := isClientVersionLessThan(resp.Version)
	if err != nil {
		return err
	}

	if ok {
		return fmt.Errorf("A new version (daemon version: %s) of this client exists, please update", resp.Version)
	}

	return nil
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

func ReadPassword() (string, error) {
	fmt.Printf("Password: ")
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	fmt.Printf("\n")
	if err != nil {
		return "", err
	}

	return string(bytePassword), nil
}

func Execute() {
	c, err := NewClient()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	if err := c.CheckVersionSync(); err != nil {
		PrintWarning(err.Error())
	}

	var rootCmd = &cobra.Command{Use: "ntp"}
	rootCmd.AddCommand(
		c.CmdVersion(),
		c.CmdUser(),
		c.CmdEvent(),
		c.CmdExercise(),
		c.CmdHost(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
