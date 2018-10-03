package cli

import (
	"context"
	"fmt"
	"log"
	"syscall"
	"time"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

func (c *Client) CmdUser() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Actions to perform on users",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(c.CmdInviteUser())
	cmd.AddCommand(c.CmdCreateUser())

	return cmd
}

func (c *Client) CmdInviteUser() *cobra.Command {
	return &cobra.Command{
		Use:   "invite",
		Short: "Create key for inviting other users",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			r, err := c.rpcClient.InviteUser(ctx, &pb.InviteUserRequest{})
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
