// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package vbox_test

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tst "github.com/aau-network-security/haaukins/testing"
	"github.com/aau-network-security/haaukins/virtual"
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

	teststart := func() {
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
	}
	teststart()

	err = vm.Suspend(ctx)
	if err != nil {
		t.Fatalf("unexpected error when suspending vm: %s", err)
	}

	output, err = execute("list", "runningvms")
	if err != nil {
		t.Fatalf("unexpected error when listing running vms: %s", err)
	}

	if strings.Contains(output, name) {
		t.Fatalf("expected virtual machine to have been stopped")
	}

	// Check state using info command
	if info := vm.Info(); info.State != virtual.Suspended {
		t.Fatalf("expected virtual machine to be suspended")
	}

	teststart()

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

func TestSetRAMandCPU(t *testing.T) {
	tst.SkipCI(t)
	ramre := regexp.MustCompile(`memory=[1-9][0-9]*`)
	cpure := regexp.MustCompile(`cpus=[0-9]*`)
	memorysize := 1024
	ctx := context.Background()
	cs := "d41d8cd98f00b204e9800998ecf8427e"
	vm := vbox.NewVMWithSum("haaukins.ova", "haaukins", cs)
	if err := vm.Create(ctx); err != nil {
		t.Fatalf("Error on creating VM : %s", err)
	}
	if err := vm.Snapshot("test_haaukins"); err != nil {
		t.Fatalf("Error on making snapshot on VM: %s", err)
	}
	defer vm.Close()
	linkedCloneVM, err := vm.LinkedClone(ctx, "test_haaukins", vbox.SetRAM(uint(memorysize)), vbox.SetCPU(uint(2)))
	if err != nil {
		t.Fatalf("Linked clone could not created %s ", err)
	}
	defer linkedCloneVM.Close()
	b, err := vbox.VBoxCmdContext(ctx, "showvminfo", linkedCloneVM.Info().Id, "--machinereadable")
	if err != nil {
		t.Fatalf("Error happened while retrieving information about ram %s", err)
	}
	cpuInfo := strings.Split(string(cpure.Find(b)), "=")
	if len(cpuInfo) != 2 {
		t.Fatalf("Splitting info error, there is something wrong with adjusting CPU")
	}
	numberOfCpu, err := strconv.Atoi(cpuInfo[1])
	if err != nil {
		t.Fatalf("Converting error string to int ... %s", err)
	}
	if numberOfCpu <= 0 {
		t.Fatalf("Error, invalid number of CPU ")
	}
	result := (strings.Split(string(ramre.Find(b)), "="))
	if len(result) == 2 {
		memSize, err := strconv.Atoi(result[1])
		if err != nil {
			t.Fatalf("Error happened while converting memory into int %s", err)
		}
		if memSize != memorysize {
			t.Fatalf("memory could not be set corretly %d", memSize)
		}
	} else {
		t.Fatalf("Error while splitting retrieved information from vboxmanage, memory in proper %s", result[1])
	}
}
