// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	color "github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"io"
)

func (c *Client) CmdHost() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host",
		Short: "Actions to perform on host",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdHostMonitor(),
	)

	return cmd
}

func formatStats(stats *pb.MonitorHostResponse, simple bool) string {
	circle := "●"
	if simple {
		circle = "<>"
	}

	cpuShow := fmt.Sprintf("%.1f", stats.CPUPercent)
	cpuShowColor := color.NewAurora(!simple || stats.CPUReadError != "")
	var cpuColor func(interface{}) color.Value
	switch {
	case stats.CPUPercent > 75:
		cpuColor = cpuShowColor.Red
	case stats.CPUPercent > 50 && stats.CPUPercent <= 75:
		cpuColor = cpuShowColor.Brown
	case stats.CPUReadError != "":
		cpuColor = cpuShowColor.Red
		cpuShow = "<err>"
	default:
		cpuColor = cpuShowColor.Green
	}

	memShow := fmt.Sprintf("%.1f", stats.MemoryPercent)
	memShowColor := color.NewAurora(!simple || stats.MemoryReadError != "")
	var memColor func(interface{}) color.Value
	switch {
	case stats.MemoryPercent > 75:
		memColor = memShowColor.Red
	case stats.MemoryPercent > 50 && stats.MemoryPercent <= 75:
		memColor = memShowColor.Brown
	case stats.CPUReadError != "":
		memColor = memShowColor.Red
		memShow = "<err>"
	default:
		memColor = memShowColor.Green
	}

	return fmt.Sprintf("\r%s CPU: %s%%%%    %s MEM: %s%%%%",
		cpuColor(circle),
		cpuShow,
		memColor(circle),
		memShow,
	)
}

func (c *Client) CmdHostMonitor() *cobra.Command {
	var simple bool

	cmd := &cobra.Command{
		Use:     "monitor",
		Short:   "Monitor host resources",
		Example: `hkn host monitor`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			stream, err := c.rpcClient.MonitorHost(ctx, &pb.Empty{})
			if err != nil {
				PrintError(err)
				return
			}
			for {
				stats, err := stream.Recv()
				if err == io.EOF {
					break
				}

				fmt.Printf(formatStats(stats, simple))
			}
		},
	}

	cmd.Flags().BoolVarP(&simple, "simple", "s", false, "show simpler and pure ascii output")

	return cmd
}
