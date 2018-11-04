package main

import (
	"fmt"
	"github.com/giantswarm/semver-bump/bump"
	"github.com/giantswarm/semver-bump/storage"
	"github.com/spf13/cobra"
	"os"
)

var (
	versionFile = "VERSION"
)

func newSemverBumper() (*bump.SemverBumper, error) {
	vs, err := storage.NewVersionStorage("file", "")
	if err != nil {
		return nil, err
	}
	return bump.NewSemverBumper(vs, versionFile), nil
}

func patch() *cobra.Command {
	return &cobra.Command{
		Use:   "patch",
		Short: "Release a patch version",
		Run: func(cmd *cobra.Command, args []string) {
			sb, err := newSemverBumper()
			if err != nil {
				fmt.Printf("Failed to create semver bumper: %s", err)
				return
			}
			curVer, err := sb.GetCurrentVersion()
			if err != nil {
				fmt.Printf("Failed to get current version: %s", err)
				return
			}
			newVer, err := sb.BumpPatchVersion("", "")
			if err != nil {
				fmt.Printf("Failed to bump version: %s", err)
				return
			}
			fmt.Printf("Releasing version %s (from %s)", curVer.String(), newVer.String())
		},
	}
}

func main() {
	var rootCmd = &cobra.Command{Use: "release"}
	rootCmd.AddCommand(
		patch(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
