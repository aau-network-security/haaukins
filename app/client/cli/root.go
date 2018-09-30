package cli

import (
	"context"
	"log"
	"time"
    "fmt"
    "os"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/spf13/cobra"
)

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

func Execute() {
	c, err := NewClient()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

    var rootCmd = &cobra.Command{Use: "ntp"}
	rootCmd.AddCommand(c.CmdLogin())
	rootCmd.AddCommand(c.CmdUser())
	rootCmd.AddCommand(c.CmdEvent())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
