package main

import (
	"context"
	"fmt"
	"os/exec"
)

var (
	bcs = []buildContext{
		{Os: DARWIN, App: CLIENT},
		{Os: LINUX, App: CLIENT},
		{Os: WINDOWS, App: CLIENT},
		{Os: DARWIN, App: DAEMON},
		{Os: LINUX, App: DAEMON},
	}

	appname = map[string]string{
		"client": "ntp",
		"daemon": "ntpd",
	}

	WINDOWS = os{"windows", ".exe"}
	LINUX   = os{"linux", ""}
	DARWIN  = os{"darwin", ""}

	DAEMON = app{"daemon", "ntpd"}
	CLIENT = app{"client", "ntp"}
)

type os struct {
	Name      string
	Extension string
}

type app struct {
	Subdirectory   string
	FilenamePrefix string
}

type buildContext struct {
	Arch string
	Os   os
	App  app
}

func (bc *buildContext) outputFileName() string {
	return fmt.Sprintf("%s-%s-%s%s", bc.App.FilenamePrefix, bc.Os.Name, bc.Arch, bc.Os.Extension)
}

func (bc *buildContext) outputFilePath() string {
	return fmt.Sprintf("./build/%s", bc.outputFileName())
}

func (bc *buildContext) packageName() string {
	return fmt.Sprintf("github.com/aau-network-security/go-ntp/app/%s", bc.App.Subdirectory)
}

func (bc *buildContext) build(ctx context.Context) error {
	cmd := exec.CommandContext(
		ctx,
		"env",
		fmt.Sprintf("GOOS=%s", bc.Os.Name),
		fmt.Sprintf("GARCH=%s", bc.Arch),
		"go",
		"build",
		"-o",
		bc.outputFilePath(),
		bc.packageName(),
	)
	_, err := cmd.CombinedOutput()
	return err
}

func main() {
	ctx := context.Background()
	for _, bc := range bcs {
		for _, arch := range []string{"amd64", "386"} {
			bcWithArch := buildContext{arch, bc.Os, bc.App}
			if err := bcWithArch.build(ctx); err != nil {
				fmt.Printf("\u2717 %s: %+v\n", bcWithArch.outputFileName(), err)
				continue
			}
			fmt.Printf("\u2713 %s\n", bcWithArch.outputFileName())
		}
	}
}
