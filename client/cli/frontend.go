// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"
	"time"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/spf13/cobra"
)

func (c *Client) CmdFrontend() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "frontend",
		Short: "Actions to perform on frontends",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdFrontendList(),
		c.CmdFrontendReset(),
		c.CmdFrontendSet(),
	)

	return cmd
}

func (c *Client) CmdFrontends() *cobra.Command {
	return &cobra.Command{
		Use:     "frontends",
		Short:   "List available frontends",
		Example: `hkn frontend list`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			r, err := c.rpcClient.ListFrontends(ctx, &pb.Empty{})
			if err != nil {
				PrintError(err)
				return
			}

			f := formatter{
				header: []string{"IMAGE NAME", "SIZE", "MEMORY (MB)", "CPU"},
				fields: []string{"Image", "Size", "MemoryMB", "Cpu"},
			}

			var elements []formatElement
			for _, f := range r.Frontends {
				memoryStr := fmt.Sprintf("%d", f.MemoryMB)
				if f.MemoryMB == 0 {
					memoryStr = "-"
				}
				cpuStr := fmt.Sprintf("%f", f.Cpu)
				if f.Cpu == 0 {
					cpuStr = "-"
				}
				elements = append(elements, struct {
					Image    string
					Size     int64
					MemoryMB string
					Cpu      string
				}{
					Image:    f.Image,
					Size:     f.Size,
					MemoryMB: memoryStr,
					Cpu:      cpuStr,
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

func (c *Client) CmdFrontendList() *cobra.Command {
	cmd := *c.CmdFrontends()
	cmd.Use = "ls"
	cmd.Aliases = []string{"ls", "list"}
	return &cmd
}

func (c *Client) CmdFrontendReset() *cobra.Command {
	var (
		teamIds []string
		teams   []*pb.Team
	)

	cmd := &cobra.Command{
		Use:     "reset [event tag]",
		Short:   "Reset frontends",
		Long:    "Reset frontends, use -t for specifying certain teams only.",
		Example: `hkn frontend reset demo -t d11eb89b`,
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			for _, t := range teamIds {
				teams = append(teams, &pb.Team{Id: t})
			}

			evTag := args[0]
			stream, err := c.rpcClient.ResetFrontends(ctx, &pb.ResetFrontendsRequest{
				EventTag: evTag,
				Teams:    teams,
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

	cmd.Flags().StringSliceVarP(&teamIds, "teams", "t", nil, "list of team ids for which to reset their frontends")

	return cmd
}

func (c *Client) CmdFrontendSet() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set a default property of a frontend",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdFrontendSetMemory(),
		c.CmdFrontendSetCpu())

	return cmd
}

func (c *Client) CmdFrontendSetMemory() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "memory [image] [memory mb]",
		Short: "Set the default RAM (in MB) of a frontend",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			image := args[0]
			memoryMB, err := strconv.Atoi(args[1])
			if err != nil {
				PrintError(err)
				return
			}

			req := &pb.SetFrontendMemoryRequest{
				Image:    image,
				MemoryMB: int64(memoryMB),
			}

			if _, err := c.rpcClient.SetFrontendMemory(ctx, req); err != nil {
				PrintError(err)
				return
			}
		},
	}

	return cmd
}

func (c *Client) CmdFrontendSetCpu() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cpu [image] [cpu count]",
		Short: "Set the default CPU count of a frontend",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			image := args[0]
			cpu, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				PrintError(err)
				return
			}

			req := &pb.SetFrontendCpuRequest{
				Image: image,
				Cpu:   float32(cpu),
			}

			if _, err := c.rpcClient.SetFrontendCpu(ctx, req); err != nil {
				PrintError(err)
				return
			}
		},
	}

	return cmd
}
