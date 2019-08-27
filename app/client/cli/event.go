// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"errors"
	"fmt"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	pbar "github.com/schollz/progressbar"
	"github.com/spf13/cobra"
	"io"
	"time"
)

var (
	UnableCreateEListErr = errors.New("Failed to create event list")
)

func (c *Client) CmdEvent() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event",
		Short: "Actions to perform on events",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdEventCreate(),
		c.CmdEventStop(),
		c.CmdEventList(),
		c.CmdEventTeams(),
		c.CmdEventTeamRestart())

	return cmd
}

func (c *Client) CmdEventCreate() *cobra.Command {
	var (
		name      string
		available int
		capacity  int
		frontends []string
		exercises []string
	)

	cmd := &cobra.Command{
		Use:     "create [event tag]",
		Short:   "Create event",
		Example: `hkn event create esboot -name "ES Bootcamp" -a 5 -c 30 -e scan,sql,hb -f kali`,
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			tag := args[0]
			stream, err := c.rpcClient.CreateEvent(ctx, &pb.CreateEventRequest{
				Name:      name,
				Tag:       tag,
				Frontends: frontends,
				Exercises: exercises,
				Capacity:  int32(capacity),
				Available: int32(available),
			})
			if err != nil {
				PrintError(err)
				return
			}
			// progress bar library changed
			// now it does not create stack of progress bar,
			// once anything is received from daemon.
			bar := pbar.New(available)
			bar.RenderBlank()
			for {
				labStatus, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					PrintError(err)
					return
				}
				if labStatus.ErrorMessage != "" {
					fmt.Println(labStatus.ErrorMessage)
					// Once we have got error, error message will be displayed
					// and support information will be shown on client terminal.
					// todo : it might be good idea to create unique case id and print it out to client
					// todo : once user is trying to contact with us they can communicate with error message and case id.
					// sometime, it is not required to shutdown event from scratch if any error occured during cloning VM,
					// server might also can send notification about the error.
				}
				bar.Add(1)
			}
			bar.Finish()
		},
	}

	cmd.Flags().StringVarP(&name, "name", "n", "", "the event name")
	cmd.Flags().IntVarP(&available, "available", "a", 5, "amount of labs to make available initially for the event")
	cmd.Flags().IntVarP(&capacity, "capacity", "c", 10, "maximum amount of labs")
	cmd.Flags().StringSliceVarP(&frontends, "frontends", "f", []string{}, "list of frontends to have for each lab")
	cmd.Flags().StringSliceVarP(&exercises, "exercises", "e", []string{}, "list of exercises to have for each lab")
	cmd.MarkFlagRequired("name")

	return cmd
}

func (c *Client) CmdEventStop() *cobra.Command {
	return &cobra.Command{
		Use:     "stop [event tag]",
		Short:   "Stop event",
		Example: `hkn event stop esboot`,
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tag := args[0]
			stream, err := c.rpcClient.StopEvent(ctx, &pb.StopEventRequest{
				Tag: tag,
			})

			if err != nil {
				PrintError(err)
				return
			}

			for {
				_, err := stream.Recv()
				if err == io.EOF {
					break
				}

				if err != nil {
					PrintError(err)
					return
				}
			}

		},
	}
}

func (c *Client) CmdEvents() *cobra.Command {
	return &cobra.Command{
		Use:     "events",
		Short:   "List events",
		Example: `hkn event list`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			r, err := c.rpcClient.ListEvents(ctx, &pb.ListEventsRequest{})
			if err != nil {
				PrintError(err)
				return
			}

			f := formatter{
				header: []string{"EVENT TAG", "NAME", "# TEAM", "# EXERCISES", "CAPACITY", "CREATION TIME"},
				fields: []string{"Tag", "Name", "TeamCount", "ExerciseCount", "Capacity", "CreationTime"},
			}

			var elements []formatElement
			for _, e := range r.Events {
				elements = append(elements, e)
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

func (c *Client) CmdEventList() *cobra.Command {
	cmd := *c.CmdEvents()
	cmd.Use = "ls"
	cmd.Aliases = []string{"ls", "list"}
	return &cmd
}

func (c *Client) CmdEventTeams() *cobra.Command {
	return &cobra.Command{
		Use:     "teams [event tag]",
		Short:   "Get teams for a event",
		Example: `hkn event teams esboot`,
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			tag := args[0]
			r, err := c.rpcClient.ListEventTeams(ctx, &pb.ListEventTeamsRequest{
				Tag: tag,
			})

			if err != nil {
				PrintError(err)
				return
			}

			f := formatter{
				header: []string{"TEAM ID", "NAME", "EMAIL"},
				fields: []string{"Id", "Name", "Email"},
			}

			var elements []formatElement
			for _, e := range r.Teams {
				elements = append(elements, e)
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

func (c *Client) CmdEventTeamRestart() *cobra.Command {
	return &cobra.Command{
		Use:     "restart [event tag] [team id]",
		Short:   "Restart lab for a team",
		Example: `hkn event restart esboot d11eb89b`,
		Args:    cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			eventTag := args[0]
			teamId := args[1]

			stream, err := c.rpcClient.RestartTeamLab(ctx, &pb.RestartTeamLabRequest{
				EventTag: eventTag,
				TeamId:   teamId,
			})

			if err != nil {
				PrintError(err)
				return
			}

			for {
				_, err := stream.Recv()
				if err == io.EOF {
					break
				}

				if err != nil {
					PrintError(err)
					return
				}
			}

		},
	}
}
