package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"time"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc"
)

type Token string

func (t Token) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"token": string(t),
	}, nil
}

func (t Token) RequireTransportSecurity() bool {
	return true
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

	conn, err := grpc.Dial("cli.sec-aau.dk:5454",
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

func (c *Client) CmdLogin() *cobra.Command {
	return &cobra.Command{
		Use:   "login [username]",
		Short: "Login user",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			username := args[0]
			password, err := ReadPassword()
			if err != nil {
				log.Fatal("Unable to read password")
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			r, err := c.rpcClient.Login(ctx, &pb.LoginRequest{
				Username: username,
				Password: password,
			})
			if err != nil {
				PrintError(r.Error)
				return
			}

			if r.Error != "" {
				PrintError(r.Error)
				return
			}

			c.Token = r.Token

			if err := c.SaveToken(); err != nil {
				PrintError(err.Error())
			}
		},
	}
}

func (c *Client) CmdCreateSignupKey() *cobra.Command {
	return &cobra.Command{
		Use:   "invite",
		Short: "Create key for inviting other users",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			r, err := c.rpcClient.CreateSignupKey(ctx, &pb.CreateSignupKeyRequest{})
			if err != nil {
				PrintError(err.Error())
				return
			}

			fmt.Println(r.Key)
		},
	}
}

func (c *Client) CmdCreateUser() *cobra.Command {
	return &cobra.Command{
		Use:   "signup [key]",
		Short: "Signup as user",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Print("Username: ")
			var username string
			fmt.Scanln(&username)

			password, err := ReadPassword()
			if err != nil {
				log.Fatal("Unable to read password")
			}

			fmt.Printf("Password (again): ")
			bytePass2, err := terminal.ReadPassword(int(syscall.Stdin))
			if err != nil {
				log.Fatal("Unable to read password")
			}
			fmt.Printf("\n")

			pass2 := string(bytePass2)
			if password != pass2 {
				PrintError("Passwords do not match, so cancelling signup :-(")
			}
			key := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			r, err := c.rpcClient.CreateUser(ctx, &pb.CreateUserRequest{
				Key:      key,
				Username: username,
				Password: password,
			})
			if err != nil {
				PrintError(err.Error())
				return
			}

			c.Token = r.Token
			if err := c.SaveToken(); err != nil {
				PrintError(err.Error())
			}
		},
	}
}

func (c *Client) CmdEventCreate() *cobra.Command {
	var buffer int
	var capacity int
	var frontends []string
	var exercises []string

	cmd := &cobra.Command{
		Use:   "create [name] [tag]",
		Short: "Create event",
		Args:  cobra.MinimumNArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			name, tag := args[0], args[1]
			stream, err := c.rpcClient.CreateEvent(ctx, &pb.CreateEventRequest{
				Name:      name,
				Tag:       tag,
				Frontends: frontends,
				Exercises: exercises,
				Capacity:  int32(capacity),
				Buffer:    int32(capacity),
			})
			if err != nil {
				PrintError(err.Error())
				return
			}

			for {
				_, err := stream.Recv()
				if err == io.EOF {
					break
				}

				if err != nil {
					PrintError(err.Error())
					return
				}
			}

		},
	}

	cmd.Flags().IntVarP(&buffer, "buffer", "b", 2, "amount of lab hubs to buffer")
	cmd.Flags().IntVarP(&capacity, "capacity", "c", 10, "capacity of total amount of labs")
	cmd.Flags().StringSliceVarP(&frontends, "frontends", "f", []string{}, "list of frontends to have for each lab")
	cmd.Flags().StringSliceVarP(&exercises, "exercises", "e", []string{}, "list of exercises to have for each lab")

	return cmd
}

func (c *Client) CmdEventStop() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [tag]",
		Short: "Stop event",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tag := args[0]
			stream, err := c.rpcClient.StopEvent(ctx, &pb.StopEventRequest{
				Tag: tag,
			})
			if err != nil {
				PrintError(err.Error())
				return
			}

			for {
				_, err := stream.Recv()
				if err == io.EOF {
					break
				}

				if err != nil {
					PrintError(err.Error())
					return
				}
			}

		},
	}
}

func (c *Client) CmdUser() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Actions to perform on users",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(c.CmdCreateSignupKey())
	cmd.AddCommand(c.CmdCreateUser())

	return cmd
}

func (c *Client) CmdEvent() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event",
		Short: "Actions to perform on events",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(c.CmdEventCreate())
	cmd.AddCommand(c.CmdEventStop())

	return cmd
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

func main() {
	c, err := NewClient()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	var rootCmd = &cobra.Command{Use: "ntp"}
	rootCmd.AddCommand(c.CmdLogin())
	rootCmd.AddCommand(c.CmdUser())
	rootCmd.AddCommand(c.CmdEvent())
	rootCmd.Execute()
}
