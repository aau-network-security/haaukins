// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/spf13/cobra"
)

var Version string

type IncorrectVersonFmt struct {
	src string
	fmt string
}

func (ivf *IncorrectVersonFmt) Error() string {
	return fmt.Sprintf("Unexpected version format (version: \"%s\") from %s", ivf.fmt, ivf.src)
}

func isClientVersionLessThan(srv string) (bool, error) {
	if Version == "" {
		return false, nil
	}

	cliParts := strings.Split(Version, ".")
	srvParts := strings.Split(srv, ".")

	if len(cliParts) < 2 {
		return false, &IncorrectVersonFmt{src: "client", fmt: Version}
	}

	if len(srvParts) < 2 {
		fmt.Println("parts")
		return false, &IncorrectVersonFmt{src: "daemon", fmt: srv}
	}

	intCliV, err := strconv.Atoi(strings.Join(cliParts[0:2], ""))
	if err != nil {
		return false, &IncorrectVersonFmt{src: "client", fmt: Version}
	}

	intSrvV, err := strconv.Atoi(strings.Join(srvParts[0:2], ""))
	if err != nil {
		return false, &IncorrectVersonFmt{src: "daemon", fmt: Version}
	}

	if intSrvV > intCliV {
		return true, nil
	}

	return false, nil
}

func (c *Client) CmdVersion() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(
		c.CmdVersionClient(),
		c.CmdVersionDaemon())

	return cmd
}

func (c *Client) CmdVersionClient() *cobra.Command {
	return &cobra.Command{
		Use:     "client",
		Short:   "Print client version",
		Example: `hkn version client`,
		Run: func(cmd *cobra.Command, args []string) {
			if Version == "" {
				fmt.Printf("client: undefined\n")
				return
			}
			fmt.Printf("client: %s\n", Version)
		},
	}
}

func (c *Client) CmdVersionDaemon() *cobra.Command {
	return &cobra.Command{
		Use:     "daemon",
		Short:   "Print daemon version",
		Example: `hkn version daemon`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()

			resp, err := c.rpcClient.Version(ctx, &pb.Empty{})
			if err != nil {
				PrintError(err)
				return
			}

			if resp.Version == "" {
				fmt.Printf("daemon: undefined\n")
				return
			}

			fmt.Printf("daemon: %s\n", resp.Version)
		},
	}
}
