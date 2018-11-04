package cli

import (
	"context"
	"fmt"
	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/spf13/cobra"
	"strconv"
	"time"
)

func (c *Client) CmdFrontend() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "frontend",
		Short: "Actions to perform on frontends",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdFrontendList(),
		c.CmdFrontendSet())

	return cmd
}

func (c *Client) CmdFrontends() *cobra.Command {
	return &cobra.Command{
		Use:     "frontends",
		Short:   "List available frontends",
		Example: `  ntp frontend list`,
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
