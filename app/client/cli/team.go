package cli

import (
	"github.com/spf13/cobra"
	"time"
	"context"
	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"fmt"
)

func (c *Client) CmdTeam() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Actions to perform on teams",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdTeamInfo(),
	)

	return cmd
}

func (c *Client) CmdTeamInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "info [team id] [event tag]",
		Short:   "Get the info of a team",
		Example: "ntp team describe azbu29c1 test-event",
		Args:    cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			teamId := args[0]
			eventTag := args[1]
			req := &pb.GetTeamInfoRequest{
				TeamId: teamId,
				EventTag: eventTag,
			}
			resp, err := c.rpcClient.GetTeamInfo(ctx, req)
						if err != nil {
				PrintError(err)
				return
			}

			f := formatter{
				header: []string{"IMAGE NAME", "TYPE", "ID"},
				fields: []string{"Image", "Type", "Id"},
			}

			var elements []formatElement
			for _, i := range resp.Instances {
				elements = append(elements, i)
			}

			table, err := f.AsTable(elements)
			if err != nil {
				PrintError(UnableCreateEListErr)
				return
			}
			fmt.Printf(table)
		},
	}

	return cmd
}