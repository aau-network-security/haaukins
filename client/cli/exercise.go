// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/spf13/cobra"
)

func (c *Client) CmdExercise() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exercise",
		Short: "Actions to perform on exercises",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdExerciseList(),
		c.CmdExerciseReset(),
		c.CmdAddExercise(),
	)

	return cmd
}

func (c *Client) CmdExercises() *cobra.Command {
	return &cobra.Command{
		Use:     "exercises",
		Short:   "List exercises",
		Example: `hkn exercise list`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			r, err := c.rpcClient.ListExercises(ctx, &pb.Empty{})
			if err != nil {
				PrintError(err)
				return
			}

			f := formatter{
				header: []string{"NAME", "TAGS", "Secret", "# DOCKER IMAGES", "# VBOX IMAGES"},
				fields: []string{"Name", "Tags", "Secret", "DockerImageCount", "VboxImageCount"},
			}

			var elements []formatElement
			for _, e := range r.Exercises {
				elements = append(elements, struct {
					Name             string
					Tags             string
					DockerImageCount int32
					VboxImageCount   int32
					Secret           bool
				}{
					Name:             e.Name,
					Tags:             strings.Join(e.Tags, ","),
					DockerImageCount: e.DockerImageCount,
					VboxImageCount:   e.VboxImageCount,
					Secret:           e.Secret,
				})
			}

			table, err := f.AsTable(elements)
			if err != nil {
				PrintError(UnableCreateEListErr)
				return
			}
			fmt.Printf(table)
		},
	}
}

func (c *Client) CmdExerciseList() *cobra.Command {
	cmd := *c.CmdExercises()
	cmd.Use = "ls"
	cmd.Aliases = []string{"ls", "list"}
	return &cmd
}

func (c *Client) CmdExerciseReset() *cobra.Command {
	var (
		evTag   string
		teamIds []string
		teams   []*pb.Team
	)

	cmd := &cobra.Command{
		Use:     "reset [exercise tag]",
		Short:   "Reset exercises",
		Long:    "Reset exercises, use -t for specifying certain teams only.",
		Example: `hkn reset sql -e esboot -t d11eb89b`,
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			for _, t := range teamIds {
				teams = append(teams, &pb.Team{Id: t})
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
				msg, err := stream.Recv()
				if err == io.EOF {
					break
				}

				if err != nil {
					log.Fatalf(err.Error())
				}

				fmt.Printf("[%s] %s\n", msg.Status, msg.TeamId)
			}
		},
	}

	cmd.Flags().StringVarP(&evTag, "evtag", "e", "", "the event name")
	cmd.Flags().StringSliceVarP(&teamIds, "teams", "t", nil, "list of team ids for which to reset the exercise")
	cmd.MarkFlagRequired("evtag")

	return cmd
}

func (c *Client) CmdAddExercise() *cobra.Command {
	var (
		evTag  string
		exTags []string
	)

	cmd := &cobra.Command{
		Use:     "add -e [event tag] -x [exercise tags]",
		Short:   "Add exercises",
		Long:    "Add exercise to given event",
		Example: `hkn add -e bootcamp -x sql,scan,mitm`,
		Args:    cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			resp, err := c.rpcClient.AddChallenge(ctx, &pb.AddChallengeRequest{
				EventTag:     evTag,
				ChallengeTag: exTags,
			})

			if err != nil {
				PrintError(err)
				return
			}
			for {
				msg, err := resp.Recv()
				if err == io.EOF {
					break
				}

				if err != nil {
					log.Fatalf(err.Error())
				}

				fmt.Printf("AddChallenge Response: %s \n", msg.Message)
			}

		},
	}

	cmd.Flags().StringVarP(&evTag, "evtag", "e", "", "the event tag")
	cmd.Flags().StringSliceVarP(&exTags, "exercises", "x", nil, "list of exercise tags to be added to event")
	cmd.MarkFlagRequired("evtag")

	return cmd
}
