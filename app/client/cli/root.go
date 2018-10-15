package cli

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"log"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc"
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

	creds := Creds{Token: c.Token}

	ssl := os.Getenv("NTP_SSL_OFF")
	if strings.ToLower(ssl) == "true" {
		creds.Insecure = true
	}

	endpoint := fmt.Sprintf("%s:%s", host, port)
	conn, err := grpc.Dial(endpoint,
		grpc.WithInsecure(),
		grpc.WithPerRPCCredentials(creds),
	)
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

func PrintError(s string) {
	fmt.Printf("[!] %s\n", s)
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

	var rootCmd = &cobra.Command{Use: "ntp"}
	rootCmd.AddCommand(
		c.CmdVersion(),
		c.CmdUser(),
		c.CmdEvent(),
		c.CmdExercise())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
