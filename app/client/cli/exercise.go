package cli

import (
	"context"
	"fmt"
	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/spf13/cobra"
	"io"
	"log"
	"time"
)

func (c *Client) CmdExercise() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exercise",
		Short: "Actions to perform on exercises",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdExerciseReset())

	return cmd
}

func (c *Client) CmdExerciseReset() *cobra.Command {
	var (
		evTag   string
		teamIds []string
		teams   []*pb.ResetExerciseRequest_Team
	)

	cmd := &cobra.Command{
		Use:   "reset [extag]",
		Short: "Reset an exercise",
		Long:  "Reset an exercise. When no team ids are provided, the exercise is reset for all teams.",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			for _, t := range teamIds {
				teams = append(teams, &pb.ResetExerciseRequest_Team{Id: t})
			}

			exTag := args[0]
			stream, err := c.rpcClient.ResetExercise(ctx, &pb.ResetExerciseRequest{
				ExerciseTag: exTag,
				EventTag:    evTag,
				Teams:       teams,
			})
			if err != nil {
				PrintError(err)
				return
			}

			for {
				status, err := stream.Recv()
				if err == io.EOF {
					break
				}

				if err != nil {
					log.Fatalf(err.Error())
				}
				fmt.Printf("\u2713 %s\n", status.TeamId)
			}
		},
	}

	cmd.Flags().StringVarP(&evTag, "evtag", "e", "", "the event name")
	cmd.Flags().StringSliceVarP(&teamIds, "teams", "t", nil, "list of teams for which to reset the exercise")
	cmd.MarkFlagRequired("evtag")

	return cmd
}
