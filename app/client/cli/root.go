package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"
)

func Execute() {
	c, err := NewClient()
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	var rootCmd = &cobra.Command{Use: "ntp"}
	rootCmd.AddCommand(
		c.CmdUser(),
		c.CmdEvent())

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
