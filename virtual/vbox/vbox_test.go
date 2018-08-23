package vbox_test

import (
	"fmt"
	"testing"
    "os/exec"

    "github.com/aau-network-security/go-ntp/virtual/vbox"
    "github.com/rs/zerolog/log"
    "github.com/stretchr/testify/assert"
)

const (
	vboxBin     = "VBoxManage"
	vboxModVM   = "modifyvm"
	vboxStartVM = "startvm"
	vboxCtrlVM  = "controlvm"
)


func init() {
    fmt.Println("Init function!")
    log.Debug().Msg("Init..")
}

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
    // new vm
    vm, err := vbox.NewVMFromOVA("go-ntp-ova.ova", "go-ntp")
    assert.Equal(t, err, nil)

    // check if it is created
    cmd, err := execute("list", "vms")
    assert.Equal(t, err, nil)
    assert.Contains(t, cmd, "\"go-ntp\"") 

    // start vm
    err = vm.Start()
    assert.Equal(t, err, nil)

    // check if it is running
    cmd, err = execute("list", "runningvms")
    assert.Equal(t, err, nil)
    assert.Contains(t, cmd, "\"go-ntp\"") 

    // restart vm??
    err = vm.Restart()
    assert.Equal(t, err, nil)

    // check if it is running
    cmd, err = execute("list", "runningvms")
    assert.Equal(t, err, nil)
    assert.Contains(t, cmd, "\"go-ntp\"") 

    // stop vm
    err = vm.Stop()
    assert.Equal(t, err, nil)

    // check if it is running
    cmd, err = execute("list", "runningvms")
    assert.Equal(t, err, nil)
    assert.NotContains(t, cmd, "\"go-ntp\"") 

    // kill vm
    err = vm.Close()
    assert.Equal(t, err, nil)

    // check if it exists
    cmd, err = execute("list", "vms")
    assert.Equal(t, err, nil)
    assert.NotContains(t, cmd, "\"go-ntp\"") 

}
