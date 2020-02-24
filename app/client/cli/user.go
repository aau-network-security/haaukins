// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/spf13/cobra"
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
	var isNonPrivUser bool
	cmd := &cobra.Command{
		Use:     "invite",
		Short:   "Create key for inviting other users such as (superusers and members) ",
		Example: `hkn user invite --superuser or hkn user invite --nonprivuser`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			r, err := c.rpcClient.InviteUser(ctx, &pb.InviteUserRequest{SuperUser: superUser,NonPrivUser: isNonPrivUser})
			if err != nil {
				PrintError(err)
				return
			}

			if r.Error != "" {
				PrintError(fmt.Errorf(r.Error))
				return
			}

			fmt.Println(r.Key)
		},
	}
	// superuser : admin has all privileges
	// nonprivuser: has limited privilege, can only see his own events and only create one event.
	// user : has all privileges except inviting users
	cmd.Flags().BoolVarP(&isNonPrivUser, "nonprivuser", "m",false, "indicates if the signup key will create nonprivuser with limited privilege or not" )
	cmd.Flags().BoolVarP(&superUser, "super-user", "s", false, "indicates if the signup key will create a super user")
	return cmd
}

func (c *Client) CmdSignupUser() *cobra.Command {
	return &cobra.Command{
		Use:     "signup",
		Short:   "Signup as user",
		Example: `hkn user signup`,
		Run: func(cmd *cobra.Command, args []string) {
			var (
				username  string
				signupKey string
			)

			fmt.Print("Signup key: ")
			fmt.Scanln(&signupKey)

			fmt.Print("Username: ")
			fmt.Scanln(&username)

			password, err := ReadSecret("Password: ")
			if err != nil {
				log.Fatal("Unable to read password")
			}

			password2, err := ReadSecret("Password (again): ")
			if err != nil {
				log.Fatal("Unable to read password")
			}

			if password != password2 {
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
		Use:     "login",
		Short:   "Login as user",
		Example: `hkn user login`,
		Run: func(cmd *cobra.Command, args []string) {
			var username string
			fmt.Print("Username: ")
			fmt.Scanln(&username)

			password, err := ReadSecret("Password: ")
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
