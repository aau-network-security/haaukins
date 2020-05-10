// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

var (
	bcs = []buildContext{
		{Os: DARWIN, App: CLIENT},
		{Os: LINUX, App: CLIENT},
		{Os: WINDOWS, App: CLIENT},
		{Os: LINUX, App: DAEMON},
	}

	WINDOWS = OperatingSystems{"windows", ".exe"}
	LINUX   = OperatingSystems{"linux", ""}
	DARWIN  = OperatingSystems{"darwin", ""}

	DAEMON = app{"", "hknd", "github.com/aau-network-security/haaukins"}
	CLIENT = app{"client", "hkn", "github.com/aau-network-security/haaukins/client/cli"}
)

type OperatingSystems struct {
	Name      string
	Extension string
}

type app struct {
	Subdirectory   string
	FilenamePrefix string
	ImportPath     string
}

type buildContext struct {
	Arch string
	Os   OperatingSystems
	App  app
}

func (bc *buildContext) outputFileName() string {
	return fmt.Sprintf("%s-%s-%s%s", bc.App.FilenamePrefix, bc.Os.Name, bc.Arch, bc.Os.Extension)
}

func (bc *buildContext) outputFilePath() string {
	return fmt.Sprintf("./build/%s", bc.outputFileName())
}

func (bc *buildContext) packageName() string {
	return fmt.Sprintf("github.com/aau-network-security/haaukins/%s", bc.App.Subdirectory)
}

func (bc *buildContext) linkFlags(version string) string {
	return fmt.Sprintf("-w -X %s.version=%s", bc.App.ImportPath, version)
}

func (bc *buildContext) build(ctx context.Context) error {
	cmd := exec.CommandContext(
		ctx,
		"env",
		"CGO_ENABLED=0",
		fmt.Sprintf("GOOS=%s", bc.Os.Name),
		fmt.Sprintf("GOARCH=%s", bc.Arch),
		"go",
		"build",
		"-a",
		"-tags",
		"netgo",
		"-ldflags",
		bc.linkFlags(os.Getenv("GIT_TAG")),
		"-o",
		bc.outputFilePath(),
		bc.packageName(),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println(string(out))
	}

	return err
}

func main() {
	ctx := context.Background()
	fmt.Printf("Building version %s\n", os.Getenv("GIT_TAG"))
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
