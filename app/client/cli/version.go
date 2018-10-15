package cli

import (
	"context"
	"fmt"
	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/spf13/cobra"
	"time"
)

var version string

func (c *Client) CmdVersion() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdVersionClient(),
		c.CmdVersionDaemon())

	return cmd
}

func (c *Client) CmdVersionClient() *cobra.Command {
	return &cobra.Command{
		Use:   "client",
		Short: "Print client version",
		Run: func(cmd *cobra.Command, args []string) {
			if version == "" {
				fmt.Printf("client: undefined\n")
				return
			}
			fmt.Printf("client: %s\n", version)
		},
	}
}

func (c *Client) CmdVersionDaemon() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Print daemon version",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			resp, err := c.rpcClient.Version(ctx, &pb.Empty{})
			if err != nil {
				PrintError(err.Error())
				return
			}
			if resp.Version == "" {
				fmt.Printf("daemon: undefined\n")
				return
			}
			fmt.Printf("daemon: %s\n", resp.Version)
		},
	}
}
