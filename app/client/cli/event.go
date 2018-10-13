package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/spf13/cobra"
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
		c.CmdEventGroups(),
		c.CmdEventGroupRestart())

	return cmd
}

func (c *Client) CmdEventCreate() *cobra.Command {
	var (
		name      string
		buffer    int
		capacity  int
		frontends []string
		exercises []string
	)

	cmd := &cobra.Command{
		Use:   "create [tag]",
		Short: "Create event",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			tag := args[0]
			stream, err := c.rpcClient.CreateEvent(ctx, &pb.CreateEventRequest{
				Name:      name,
				Tag:       tag,
				Frontends: frontends,
				Exercises: exercises,
				Capacity:  int32(capacity),
				Buffer:    int32(buffer),
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

	cmd.Flags().StringVarP(&name, "name", "n", "", "the event name")
	cmd.Flags().IntVarP(&buffer, "buffer", "b", 2, "amount of lab hubs to buffer")
	cmd.Flags().IntVarP(&capacity, "capacity", "c", 10, "capacity of total amount of labs")
	cmd.Flags().StringSliceVarP(&frontends, "frontends", "f", []string{}, "list of frontends to have for each lab")
	cmd.Flags().StringSliceVarP(&exercises, "exercises", "e", []string{}, "list of exercises to have for each lab")
	cmd.MarkFlagRequired("name")

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

			f := formatter{
				header: []string{"EVENT TAG", "NAME", "# GROUPS", "# EXERCISES", "CAPACITY"},
				fields: []string{"Tag", "Name", "GroupCount", "ExerciseCount", "Capacity"},
			}

			var elements []formatElement
			for _, e := range r.Events {
				elements = append(elements, e)
			}

			table, err := f.AsTable(elements)
			if err != nil {
				PrintError("Failed to create event list")
				return
			}
			fmt.Printf(table)
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
