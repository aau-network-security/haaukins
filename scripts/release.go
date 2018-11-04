package main

import (
	"fmt"
	"github.com/aau-network-security/go-ntp/scripts/git"
	"github.com/coreos/go-semver/semver"
	"github.com/giantswarm/semver-bump/bump"
	"github.com/giantswarm/semver-bump/storage"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"os"
)

var (
	versionFile = "VERSION"
	bumpMajor   = func(version *semver.Version) {
		version.BumpMajor()
	}
	bumpMinor = func(version *semver.Version) {
		version.BumpMinor()
	}
	bumpPatch = func(version *semver.Version) {
		version.BumpPatch()
	}
)

func newSemverBumper() (*bump.SemverBumper, error) {
	vs, err := storage.NewVersionStorage("file", "")
	if err != nil {
		return nil, err
	}
	return bump.NewSemverBumper(vs, versionFile), nil
}

func bumpVersion(bumpFunc func(*bump.SemverBumper) (*semver.Version, error), branchFuncs ...func(*semver.Version)) error {
	sb, err := newSemverBumper()
	if err != nil {
		return errors.Wrap(err, "failed to create semver bumper")
	}
	curVer, err := sb.GetCurrentVersion()
	if err != nil {
		return errors.Wrap(err, "failed to get current version")
	}
	newVer, err := bumpFunc(sb)
	if err != nil {
		return errors.Wrap(err, "failed to bump version")
	}

	repo, err := git.NewRepo(".")
	if err != nil {
		return errors.Wrap(err, "failed to find git repo")
	}

	fmt.Printf("Releasing version %s (from %s)\n", newVer.String(), curVer.String())

	if err := repo.Commit(newVer, versionFile); err != nil {
		return errors.Wrap(err, "failed to commit version")
	}

	if err := repo.Tag(newVer); err != nil {
		return errors.Wrap(err, "failed to create tag")
	}

	for _, bf := range branchFuncs {
		newBranchVer, err := semver.NewVersion(newVer.String())
		if err != nil {
			return errors.Wrap(err, "failed to copy version")
		}

		bf(newBranchVer)
		fmt.Printf("Creating new branch '%s'\n", newBranchVer.String())
		if err := repo.CreateBranch(newBranchVer); err != nil {
			return errors.Wrap(err, "failed to create branch")
		}
	}
	if err := repo.PushBranch(); err != nil {
		return errors.Wrap(err, "failed to push branch")
	}

	return nil
}

func major() *cobra.Command {
	return &cobra.Command{
		Use:   "major",
		Short: "Release a major version",
		Run: func(cmd *cobra.Command, args []string) {
			bumpFunc := func(sb *bump.SemverBumper) (*semver.Version, error) {
				return sb.BumpMajorVersion("", "")
			}

			if err := bumpVersion(bumpFunc, bumpMajor, bumpMinor, bumpPatch); err != nil {
				fmt.Printf("Error while bumping version: %s", err)
			}
		},
	}
}

func minor() *cobra.Command {
	return &cobra.Command{
		Use:   "minor",
		Short: "Release a minor version",
		Run: func(cmd *cobra.Command, args []string) {
			bumpFunc := func(sb *bump.SemverBumper) (*semver.Version, error) {
				return sb.BumpMinorVersion("", "")
			}

			if err := bumpVersion(bumpFunc, bumpMinor, bumpPatch); err != nil {
				fmt.Printf("Error while bumping version: %s", err)
			}
		},
	}
}

func patch() *cobra.Command {
	return &cobra.Command{
		Use:   "patch",
		Short: "Release a patch version",
		Run: func(cmd *cobra.Command, args []string) {
			bumpFunc := func(sb *bump.SemverBumper) (*semver.Version, error) {
				return sb.BumpPatchVersion("", "")
			}

			if err := bumpVersion(bumpFunc, bumpPatch); err != nil {
				fmt.Printf("Error while bumping version: %s", err)
			}
		},
	}
}

func main() {
	var rootCmd = &cobra.Command{Use: "release"}
	rootCmd.AddCommand(
		major(),
		minor(),
		patch(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
