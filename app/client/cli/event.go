package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/spf13/cobra"
)

func (c *Client) CmdEventCreate() *cobra.Command {
	var buffer int
	var capacity int
	var frontends []string
	var exercises []string

	cmd := &cobra.Command{
		Use:   "create [name] [tag]",
		Short: "Create event",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			name, tag := args[0], args[1]
			stream, err := c.rpcClient.CreateEvent(ctx, &pb.CreateEventRequest{
				Name:      name,
				Tag:       tag,
				Frontends: frontends,
				Exercises: exercises,
				Capacity:  int32(capacity),
				Buffer:    int32(capacity),
			})
			if err != nil {
				PrintError(err.Error())
				return
			}

			for {
				_, err := stream.Recv()
				if err == io.EOF {
					break
				}

				if err != nil {
					PrintError(err.Error())
					return
				}
			}

		},
	}

	cmd.Flags().IntVarP(&buffer, "buffer", "b", 2, "amount of lab hubs to buffer")
	cmd.Flags().IntVarP(&capacity, "capacity", "c", 10, "capacity of total amount of labs")
	cmd.Flags().StringSliceVarP(&frontends, "frontends", "f", []string{}, "list of frontends to have for each lab")
	cmd.Flags().StringSliceVarP(&exercises, "exercises", "e", []string{}, "list of exercises to have for each lab")

	return cmd
}

func (c *Client) CmdEventStop() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [tag]",
		Short: "Stop event",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			tag := args[0]
			stream, err := c.rpcClient.StopEvent(ctx, &pb.StopEventRequest{
				Tag: tag,
			})
			if err != nil {
				PrintError(err.Error())
				return
			}

			for {
				_, err := stream.Recv()
				if err == io.EOF {
					break
				}

				if err != nil {
					PrintError(err.Error())
					return
				}
			}

		},
	}
}

func (c *Client) CmdEventList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List events",
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			r, err := c.rpcClient.ListEvents(ctx, &pb.ListEventsRequest{})
			if err != nil {
				PrintError(err.Error())
				return
			}

			for _, event := range r.Events {
				fmt.Println(event.Name)
				fmt.Printf("- Tag: %s\n", event.Tag)
				fmt.Printf("- Buffer: %d\n", event.Buffer)
				fmt.Printf("- Capacity: %d\n", event.Capacity)
				fmt.Printf("- Frontends: \n-- %s\n", strings.Join(event.Frontends, "\n-- "))
				fmt.Printf("- Exercises: \n-- %s\n", strings.Join(event.Exercises, "\n-- "))
			}
		},
	}
}

func (c *Client) CmdEventGroups() *cobra.Command {
	return &cobra.Command{
		Use:   "groups [tag]",
		Short: "Get groups for a event",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			tag := args[0]
			r, err := c.rpcClient.ListEventGroups(ctx, &pb.ListEventGroupsRequest{
				Tag: tag,
			})

			if err != nil {
				PrintError(err.Error())
				return
			}

			for _, group := range r.Groups {
				fmt.Printf("%s\n", group.Name)
				fmt.Printf("- %s\n", group.LabTag)
			}

		},
	}
}

func (c *Client) CmdEventGroupRestart() *cobra.Command {
	return &cobra.Command{
		Use:   "restart [event tag] [group lab tag]",
		Short: "Restart lab for a group",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			eventTag := args[0]
			labTag := args[1]

			stream, err := c.rpcClient.RestartGroupLab(ctx, &pb.RestartGroupLabRequest{
				EventTag: eventTag,
				LabTag:   labTag,
			})
			if err != nil {
				PrintError(err.Error())
				return
			}

			for {
				_, err := stream.Recv()
				if err == io.EOF {
					break
				}

				if err != nil {
					PrintError(err.Error())
					return
				}
			}

		},
	}
}

func (c *Client) CmdEvent() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "event",
		Short: "Actions to perform on events",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(c.CmdEventCreate())
	cmd.AddCommand(c.CmdEventStop())
	cmd.AddCommand(c.CmdEventList())
	cmd.AddCommand(c.CmdEventGroups())
	cmd.AddCommand(c.CmdEventGroupRestart())

	return cmd
}
