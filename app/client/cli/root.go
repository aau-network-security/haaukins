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

var (
	RequireTLS = true
)

type Token string

func (t Token) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"token": string(t),
	}, nil
}

func (t Token) RequireTransportSecurity() bool {
	return RequireTLS
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

	ssl := os.Getenv("NTP_SSL_OFF")
	if strings.ToLower(ssl) == "true" {
		RequireTLS = false
	}

	endpoint := fmt.Sprintf("%s:%s", host, port)
	conn, err := grpc.Dial(endpoint,
		grpc.WithInsecure(),
		grpc.WithPerRPCCredentials(Token(c.Token)),
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
		c.CmdUser(),
		c.CmdEvent())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
