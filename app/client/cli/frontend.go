package cli

import (
	"github.com/spf13/cobra"
	"time"

	"context"
	"fmt"
	pb "github.com/aau-network-security/go-ntp/daemon/proto"
)

func (c *Client) CmdFrontend() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "frontend",
		Short: "Actions to perform on frontends",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdFrontendList())

	return cmd
}

func (c *Client) CmdFrontendList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available frontends",
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
