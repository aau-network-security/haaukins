package main

import (
	"context"
	"fmt"
	"os/exec"
)

var (
	bcs = []BuildContext{
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

	WINDOWS = Os{"windows", ".exe"}
	LINUX   = Os{"linux", ""}
	DARWIN  = Os{"darwin", ""}

	DAEMON = App{"daemon", "ntpd"}
	CLIENT = App{"client", "ntp"}
)

type Os struct {
	Name      string
	Extension string
}

type App struct {
	Subdirectory   string
	FilenamePrefix string
}

type BuildContext struct {
	Arch string
	Os   Os
	App  App
}

func (bc *BuildContext) OutputFileName() string {
	return fmt.Sprintf("%s-%s-%s%s", bc.App.FilenamePrefix, bc.Os.Name, bc.Arch, bc.Os.Extension)
}

func (bc *BuildContext) OutputFilePath() string {
	return fmt.Sprintf("./build/%s", bc.OutputFileName())
}

func (bc *BuildContext) Package() string {
	return fmt.Sprintf("github.com/aau-network-security/go-ntp/app/%s", bc.App.Subdirectory)
}

func (bc *BuildContext) Build(ctx context.Context) error {
	cmd := exec.CommandContext(
		ctx,
		"env",
		fmt.Sprintf("GOOS=%s", bc.Os.Name),
		fmt.Sprintf("GARCH=%s", bc.Arch),
		"go",
		"build",
		"-o",
		bc.OutputFilePath(),
		bc.Package(),
	)
	_, err := cmd.CombinedOutput()
	return err
}

func main() {
	ctx := context.Background()
	for _, bc := range bcs {
		for _, arch := range []string{"amd64", "386"} {
			bcWithArch := BuildContext{arch, bc.Os, bc.App}
			if err := bcWithArch.Build(ctx); err != nil {
				fmt.Printf("\u2717 %s: %+v\n", bcWithArch.OutputFileName(), err)
				continue
			}
			fmt.Printf("\u2713 %s\n", bcWithArch.OutputFileName())
		}
	}
}
