package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"syscall"
	"time"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	PasswordsNoMatchErr = errors.New("Passwords do not match, so cancelling signup :-(")
)

func (c *Client) CmdUser() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "Actions to perform on users",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdInviteUser(),
		c.CmdSignupUser(),
		c.CmdLoginUser())

	return cmd
}

func (c *Client) CmdInviteUser() *cobra.Command {
	var superUser bool
	cmd := &cobra.Command{
		Use:   "invite",
		Short: "Create key for inviting other users",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			r, err := c.rpcClient.InviteUser(ctx, &pb.InviteUserRequest{SuperUser: superUser})
			if err != nil {
				PrintError(err)
				return
			}

			fmt.Println(r.Key)
		},
	}

	cmd.Flags().BoolVarP(&superUser, "super-user", "s", false, "indicates if the signup key will create a super user")
	return cmd
}

func (c *Client) CmdSignupUser() *cobra.Command {
	return &cobra.Command{
		Use:   "signup",
		Short: "Signup as user",
		Run: func(cmd *cobra.Command, args []string) {
			var (
				username  string
				signupKey string
			)

			fmt.Print("Signup key: ")
			fmt.Scanln(&signupKey)

			fmt.Print("Username: ")
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
				PrintError(PasswordsNoMatchErr)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			r, err := c.rpcClient.SignupUser(ctx, &pb.SignupUserRequest{
				Key:      signupKey,
				Username: username,
				Password: password,
			})
			if err != nil {
				PrintError(err)
				return
			}

			if r.Error != "" {
				PrintError(fmt.Errorf(r.Error))
				return
			}

			c.Token = r.Token
			if err := c.SaveToken(); err != nil {
				PrintError(err)
			}
		},
	}
}

func (c *Client) CmdLoginUser() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Login user",
		Run: func(cmd *cobra.Command, args []string) {
			var username string
			fmt.Print("Username: ")
			fmt.Scanln(&username)

			password, err := ReadPassword()
			if err != nil {
				log.Fatal("Unable to read password")
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			r, err := c.rpcClient.LoginUser(ctx, &pb.LoginUserRequest{
				Username: username,
				Password: password,
			})

			if err != nil {
				fmt.Println(err)
				return
			}

			if r.Error != "" {
				PrintError(fmt.Errorf(r.Error))
				return
			}

			c.Token = r.Token

			if err := c.SaveToken(); err != nil {
				PrintError(err)
			}
		},
	}
}
