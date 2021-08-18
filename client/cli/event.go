// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/spf13/cobra"
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
		c.CmdEventSuspend(),
		c.CmdEventResume(),
		c.CmdEventList(),
		c.CmdEventTeams(),
		c.CmdEventLoadTest(),
		c.CmdEventTeamRestart(),
		c.CmdAddNotification())

	return cmd
}

func (c *Client) CmdEventCreate() *cobra.Command {
	var (
		name              string
		available         int
		capacity          int
		frontends         []string
		exercises         []string
		disabledExercises []string
		startTime         uint64
		finishTime        uint64
		onlyVPN           bool
		secretKey         string
	)

	cmd := &cobra.Command{
		Use:     "create [event tag]",
		Short:   "Create event",
		Example: `hkn event create esboot -name "ES Bootcamp" -a 5 -c 30 -e scan,sql,hb -f kali -d 2020-02-15 -k secretKey`,
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			tag := args[0]
			stream, err := c.rpcClient.CreateEvent(ctx, &pb.CreateEventRequest{
				Name:             name,
				Tag:              tag,
				Frontends:        frontends,
				Exercises:        exercises,
				DisableExercises: disabledExercises,
				Available:        int32(available),
				Capacity:         int32(capacity),
				OnlyVPN:          0,
				StartTime:        time.Now().AddDate(0, 0, int(startTime)).Format("2006-01-02 15:04:05"),
				FinishTime:       time.Now().AddDate(0, 0, int(finishTime)).Format("2006-01-02 15:04:05"),
				SecretEvent:      secretKey,
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

	cmd.Flags().StringVarP(&name, "name", "n", "", "the event name")
	cmd.Flags().IntVarP(&available, "available", "a", 5, "amount of labs to make available initially for the event")
	cmd.Flags().IntVarP(&capacity, "capacity", "c", 10, "maximum amount of labs")
	cmd.Flags().BoolVarP(&onlyVPN, "vpnconn", "v", false, "enable only vpn connection")
	cmd.Flags().StringSliceVarP(&frontends, "frontends", "f", []string{}, "list of frontends to have for each lab")
	cmd.Flags().StringSliceVarP(&exercises, "exercises", "e", []string{}, "list of exercises to have for each lab")
	cmd.Flags().StringSliceVarP(&disabledExercises, "disabled-exercises", "x", []string{}, "list of disabled exercises, will be spin off by user in the event manually")
	cmd.Flags().Uint64VarP(&finishTime, "finishtime", "d", 15, "expected finish time of the event")
	cmd.Flags().Uint64VarP(&startTime, "starttime", "s", 0, "expected start time of the event")
	cmd.Flags().StringVarP(&secretKey, "secretkey", "k", "", "secret key for protecting events")
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
			ctx := context.Background()
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

func (c *Client) CmdEventSuspend() *cobra.Command {
	return &cobra.Command{
		Use:     "suspend",
		Short:   "Suspends event",
		Example: "hkn event suspend <event-tag>",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			tag := args[0]
			stream, err := c.rpcClient.SuspendEvent(ctx, &pb.SuspendEventRequest{
				EventTag: tag,
				Suspend:  true,
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

func (c *Client) CmdEventResume() *cobra.Command {
	return &cobra.Command{
		Use:     "resume",
		Short:   "Resumes event",
		Example: "hkn event resume <event-tag>",
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			tag := args[0]
			stream, err := c.rpcClient.SuspendEvent(ctx, &pb.SuspendEventRequest{
				EventTag: tag,
				Suspend:  false,
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
	var status string
	var statusID int32
	cmd := &cobra.Command{
		Use:     "events",
		Short:   "List events",
		Example: `hkn event list / hkn events --status closed `,
		Run: func(cmd *cobra.Command, args []string) {

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			statusID = checkStatus(status)
			r, err := c.rpcClient.ListEvents(ctx, &pb.ListEventsRequest{Status: statusID})
			if err != nil {
				PrintError(err)
				return
			}

			f := formatter{
				header: []string{"EVENT TAG", "NAME", "# TEAM", "EXERCISES", "CAPACITY", "STATUS", "CREATION TIME", "EXPECTED FINISH TIME", "CREATEDBY"},
				fields: []string{"Tag", "Name", "TeamCount", "Exercises", "Capacity", "Status", "CreationTime", "FinishTime", "CreatedBy"},
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
	cmd.Flags().StringVarP(&status, "status", "s", "running", "return events in given condition")
	return cmd
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
				header: []string{"TEAM ID", "NAME", "EMAIL", "LAST ACCESSED"},
				fields: []string{"Id", "Name", "Email", "AccessedAt"},
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

func (c *Client) CmdEventLoadTest() *cobra.Command {
	var eventTag string
	var numberOfTeams int32
	cmd := &cobra.Command{
		Use:     "load",
		Short:   "Apply load test on an event",
		Example: `hkn event load -t test -r 3 `,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			r, err := c.rpcClient.StressEvent(ctx, &pb.TestEventLoadReq{EventName: eventTag, NumberOfTeams: numberOfTeams})

			if err != nil {
				PrintError(err)
				return
			}
			fmt.Println(r.SignUpResult)
			return
		},
	}
	cmd.Flags().StringVarP(&eventTag, "tag", "t", "", "event tag")
	cmd.Flags().Int32VarP(&numberOfTeams, "requests", "r", 1, "number of users")
	return cmd
}

func (c *Client) CmdAddNotification() *cobra.Command {
	var message string
	var loggedInUsers bool
	cmd := &cobra.Command{
		Use:     "announce",
		Short:   "Announce something before hand to all event pages...",
		Example: `hkn event announce -m "This is just an example announcement for all users" -l true `,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			r, err := c.rpcClient.AddNotification(ctx, &pb.AddNotificationRequest{
				Message:     message,
				LoggedUsers: loggedInUsers,
			})

			if err != nil {
				PrintError(err)
				return
			}
			fmt.Println(r.Response)
			return
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "announcement message")
	cmd.Flags().BoolVarP(&loggedInUsers, "onlyloggedin", "l", false, "only logged users ")
	return cmd
}

func (c *Client) CmdEventTeamRestart() *cobra.Command {
	return &cobra.Command{
		Use:     "restart [event tag] [team id]",
		Short:   "Restart lab for a team",
		Example: `hkn event restart esboot d11eb89b`,
		Args:    cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// timeout value is removed because when command is used for an event 1 min
			// was not enough to restart all resources that team has. So, it stops all resources
			// then the communicaton between client and daemon exited after 1 min, then the stopped
			// resources could not start again properly.
			ctx := context.Background()
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

func checkStatus(status string) int32 {
	var statusID int32
	switch status {
	case "running":
		statusID = 0
	case "suspended":
		statusID = 1
	case "booked":
		statusID = 2
	case "closed":
		statusID = 3
	case "all":
		statusID = 99
	default:
		statusID = 0
	}
	return statusID
}
