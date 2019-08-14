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


func TestSetRAM(t *testing.T) {
	tst.SkipCI(t)
	re := regexp.MustCompile(`memory=[1-9]*`)
	memorysize:=256
	ctx := context.Background()
	cs := "d41d8cd98f00b204e9800998ecf8427e"
	vm := vbox.NewVMWithSum("haaukins.ova", "haaukins", cs)
	vm.Create(ctx)
	vm.Snapshot("test_haaukins")

	linkedCloneVM,err := vm.LinkedClone(ctx,"test_haaukins",vbox.SetRAM(uint(memorysize)))
	if err != nil {
		t.Fatalf("Linked clone could not created %s ", err)
	}

	b,err:= vbox.VBoxCmdContext(ctx,"showvminfo",linkedCloneVM.Info().Id,"--machinereadable")
	if err!=nil{
		t.Fatalf("Error happened while retrieving information about ram %s",err)
	}
	result := (strings.Split(string(re.Find(b)),"="))
	if len(result)==2 {
		memSize, err := strconv.Atoi(result[1])
		if err != nil {
			t.Fatalf("Error happened while converting memory into int %s", err)
		}
		if memSize != memorysize {
			t.Fatalf("memory could not be set corretly %d", memSize)

		}
		vm.Close()
		linkedCloneVM.Close()
	}else {
		t.Fatalf("Error while splitting retrieved information from vboxmanage")
	}



}
