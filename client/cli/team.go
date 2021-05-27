// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"time"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
)

func (c *Client) CmdTeam() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team",
		Short: "Actions to perform on teams",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdTeamInfo(),
		c.CmdTeamSuspend(),
		c.CmdTeamResume(),
		c.CmdSolveChallenge(),
		c.CmdTeamFlags(),
		c.CmdUpdateTeamPassword(),
		c.CmdDeleteTeam(),
	)

	return cmd
}

func stateString(state int32) string {
	circle := "●"

	a := aurora.NewAurora(true)
	var colorFunc func(interface{}) aurora.Value
	var stateStr string
	switch state {
	case 0:
		colorFunc = a.Green
		stateStr = "running"
	case 1:
		colorFunc = a.Brown
		stateStr = "not running"
	case 2:
		colorFunc = a.Yellow
		stateStr = "suspended"
	case 3:
		colorFunc = a.Red
		stateStr = "error"
	}

	return fmt.Sprintf("%s %s", colorFunc(circle), stateStr)
}

func (c *Client) CmdTeamInfo() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "info [team id] [event tag]",
		Short:   "Get the info of a team",
		Example: "hkn team describe azbu29c1 test-event",
		Args:    cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			teamId := args[0]
			eventTag := args[1]
			req := &pb.GetTeamInfoRequest{
				TeamId:   teamId,
				EventTag: eventTag,
			}
			resp, err := c.rpcClient.GetTeamInfo(ctx, req)
			if err != nil {
				PrintError(err)
				return
			}

			f := formatter{
				header: []string{"IMAGE NAME", "TYPE", "ID", "STATE"},
				fields: []string{"Image", "Type", "Id", "State"},
			}

			var elements []formatElement
			for _, i := range resp.Instances {
				state := stateString(i.State)
				el := struct {
					Image string
					Type  string
					Id    string
					State string
				}{
					Image: i.Image,
					Type:  i.Type,
					Id:    i.Id,
					State: state,
				}

				elements = append(elements, el)
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

func (c *Client) CmdTeamSuspend() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "suspend [team id] [event tag]",
		Short:   "Suspend a teams lab",
		Example: "hkn team suspend azbu29c1 test-event",
		Args:    cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			teamId := args[0]
			eventTag := args[1]
			req := &pb.SetTeamSuspendRequest{
				TeamId:   teamId,
				EventTag: eventTag,
				Suspend:  true,
			}
			_, err := c.rpcClient.SetTeamSuspend(ctx, req)
			if err != nil {
				PrintError(err)
				return
			}
		},
	}

	return cmd
}

func (c *Client) CmdTeamResume() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "resume [team id] [event tag]",
		Short:   "Resume a teams suspended lab",
		Example: "hkn team resume azbu29c1 test-event",
		Args:    cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			teamId := args[0]
			eventTag := args[1]
			req := &pb.SetTeamSuspendRequest{
				TeamId:   teamId,
				EventTag: eventTag,
				Suspend:  false,
			}
			_, err := c.rpcClient.SetTeamSuspend(ctx, req)
			if err != nil {
				PrintError(err)
				return
			}
		},
	}

	return cmd
}

func (c *Client) CmdSolveChallenge() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "solve [event tag] [team id] [challenge-tag]",
		Short:   "Solves a challenge for specified team",
		Example: "hkn team solve test-event azbu29c1 sql-1",
		Args:    cobra.MinimumNArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			teamId := args[1]
			eventTag := args[0]
			chalTag := args[2]
			req := &pb.SolveChallengeRequest{
				TeamID:       teamId,
				EventTag:     eventTag,
				ChallengeTag: chalTag,
			}

			resp, err := c.rpcClient.SolveChallenge(ctx, req)
			if err != nil {
				PrintError(err)
				return
			}
			fmt.Println(resp.Status)
		},
	}
	return cmd
}

func (c *Client) CmdUpdateTeamPassword() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update-pass [event tag] [team id] [ password ] [ password-repeat ]",
		Short:   "Update password of a team.",
		Example: "hkn team update-pass test-event azbu29c1 pass1 pass1",
		Args:    cobra.MinimumNArgs(4),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			eventTag := args[0]
			teamId := args[1]
			password := args[2]
			passwordRepeat := args[3]
			req := &pb.UpdateTeamPassRequest{
				EventTag:       eventTag,
				TeamID:         teamId,
				Password:       password,
				PasswordRepeat: passwordRepeat,
			}

			resp, err := c.rpcClient.UpdateTeamPassword(ctx, req)
			if err != nil {
				PrintError(err)
				return
			}
			fmt.Println(resp.Status)
		},
	}
	return cmd
}

func (c *Client) CmdDeleteTeam() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "del [ event-tag ] [team-id] ",
		Short:   "Delete team from event.",
		Example: "hkn team delete test-event azbu29c1",
		Args:    cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			eventTag := args[0]
			teamId := args[1]
			req := &pb.DeleteTeamRequest{
				EvTag:  eventTag,
				TeamId: teamId,
			}
			_, err := c.rpcClient.DeleteTeam(ctx, req)
			if err != nil {
				PrintError(err)
				return
			}
		},
	}
	return cmd
}

func (c *Client) CmdTeamFlags() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "flags [event tag] [team id]",
		Short:   "Get all flags on team",
		Example: "hkn team flags test-event azbu29c1",
		Args:    cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()

			teamId := args[1]
			eventTag := args[0]
			req := &pb.GetTeamInfoRequest{
				TeamId:   teamId,
				EventTag: eventTag,
			}
			resp, err := c.rpcClient.GetTeamChals(ctx, req)
			if err != nil {
				PrintError(err)
				return
			}

			f := formatter{
				header: []string{"CHALLENGE NAME", "CHALLENGE TAG", "CHALLENGE FLAG"},
				fields: []string{"ChalName", "ChalTag", "ChalFlag"},
			}

			var elements []formatElement
			for _, i := range resp.Flags {

				el := struct {
					ChalName string
					ChalTag  string
					ChalFlag string
				}{
					ChalName: i.ChallengeName,
					ChalTag:  i.ChallengeTag,
					ChalFlag: i.ChallengeFlag,
				}

				elements = append(elements, el)
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
