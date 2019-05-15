// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package vbox_test

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	tst "github.com/aau-network-security/haaukins/testing"
	"github.com/aau-network-security/haaukins/virtual/vbox"
)

const (
	vboxBin = "VBoxManage"
)

func execute(cmd string, cmds ...string) (string, error) {
	command := append([]string{cmd}, cmds...)
	c := exec.Command(vboxBin, command...)

	output, err := c.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(output[:]), nil
}

func TestVmBase(t *testing.T) {
	tst.SkipCI(t)
	ctx := context.Background()

	cs := "d41d8cd98f00b204e9800998ecf8427e"
	vm := vbox.NewVMWithSum("haaukins.ova", "haaukins", cs)
	if err := vm.Create(ctx); err != nil {
		t.Fatalf("unexpected error when creating vm: %s", err)
	}

	output, err := execute("list", "vms")
	if err != nil {
		t.Fatalf("unexpected error when listing vms: %s", err)
	}

	name := fmt.Sprintf(`"haaukins{%s}"`, cs)
	if !strings.Contains(output, name) {
		t.Fatalf("expected virtual machine to have been added")
	}

	err = vm.Start(ctx)
	if err != nil {
		t.Fatalf("unexpected error when starting vm: %s", err)
	}

	output, err = execute("list", "runningvms")
	if err != nil {
		t.Fatalf("unexpected error when listing running vms: %s", err)
	}

	if !strings.Contains(output, name) {
		t.Fatalf("expected virtual machine to be running")
	}

	if err := vm.Stop(); err != nil {
		t.Fatalf("unexpected error when stopping vm: %s", err)
	}

	output, err = execute("list", "runningvms")
	if strings.Contains(output, name) {
		t.Fatalf("expected virtual machine to have been stopped")
	}

	if err := vm.Close(); err != nil {
		t.Fatalf("unexpected error when closing vm: %s", err)
	}

	output, err = execute("list", "vms")
	if strings.Contains(output, name) {
		t.Fatalf("expected virtual machine to have been removed")
	}
}
