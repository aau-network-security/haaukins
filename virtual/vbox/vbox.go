package vbox

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	vboxBin     = "VBoxManage"
	vboxModVM   = "modifyvm"
	vboxStartVM = "startvm"
	vboxCtrlVM  = "controlvm"
)

type VM interface {
	Snapshot(string) error
	LinkedClone(string) (VM, error)
}

type vm struct {
	id      string
	running bool
}

func NewVMFromOVA(path, name string) (*vm, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, vboxBin, "import", path, "--vsys", "0", "--vmname", name)
	_, err := c.Output()
	if err != nil {
		return nil, err
	}

	return &vm{id: name}, nil
}

func (vm *vm) Start() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, vboxBin, vboxStartVM, vm.id)
	_, err := c.Output()
	if err != nil {
		return err
	}

	vm.running = true

	return nil
}

func (vm *vm) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, vboxBin, vboxCtrlVM, vm.id, "poweroff")
	_, err := c.Output()
	if err != nil {
		return err
	}

	vm.running = false

	return nil
}

func (vm *vm) Restart() error {
	if err := vm.Stop(); err != nil {
		return err
	}

	if err := vm.Start(); err != nil {
		return err
	}

	return nil
}

func (vm *vm) SetBridge(n string) error {
	return nil
}

func (vm *vm) SetRAM(mb uint) error {
	wasRunning := vm.running
	if vm.running {
		if err := vm.Stop(); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, vboxBin, vboxModVM, vm.id, "--memory", fmt.Sprintf("%d", mb))
	_, err := c.Output()
	if err != nil {
		return err
	}

	if wasRunning {
		if err := vm.Start(); err != nil {
			return err
		}
	}

	return nil
}

func (vm *vm) SetCPU(cores uint) error {
	wasRunning := vm.running
	if vm.running {
		if err := vm.Stop(); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, vboxBin, vboxModVM, vm.id, "--cpus", fmt.Sprintf("%d", cores))
	_, err := c.Output()
	if err != nil {
		return err
	}

	if wasRunning {
		if err := vm.Start(); err != nil {
			return err
		}
	}

	return nil
}

func (vm *vm) Snapshot(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, vboxBin, "snapshot", vm.id, "take", name)
	_, err := c.Output()
	if err != nil {
		return err
	}

	return nil
}

func (*vm) LinkedClone(snapshot string) (VM, error) {
	newID := uuid.New().String()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, vboxBin, "clonevm", "--snapshot", snapshot, "--options", "link", "--name", newID, "--register")
	_, err := c.Output()
	if err != nil {
		return nil, err
	}

	return &vm{id: newID}, nil
}

type Library interface {
	GetCopy(string) (VM, error)
}

type vBoxLibrary struct {
	m     sync.Mutex
	known map[string]VM
	locks map[string]*sync.Mutex
}

func (lib *vBoxLibrary) GetCopy(path string) (*vm, error) {
	lib.m.Lock()

	pathLock, ok := lib.locks[path]
	if !ok {
		pathLock = &sync.Mutex{}
		lib.locks[path] = pathLock
	}

	log.Debug().
		Str("path", path).
		Bool("first_time", ok == false).
		Msg("getting path lock")

	lib.m.Unlock()

	pathLock.Lock()
	defer pathLock.Unlock()

	vm, ok := lib.known[path]
	if ok {
		return vm.LinkedClone("origin")
	}

	sum, err := checksum(path)
	if err != nil {
		return nil, err
	}

	n := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	id := fmt.Sprintf("%s{%s}", n, sum)

	vm, ok = vmExists(id)
	if !ok {
		vm, err = NewVMFromOVA(path, id)
		if err != nil {
			return nil, err
		}

		err = vm.Snapshot("origin")
		if err != nil {
			return nil, err
		}
	}

	lib.m.Lock()
	lib.known[path] = vm
	lib.m.Unlock()

	return vm.LinkedClone("origin")
}

func checksum(filepath string) (string, error) {
	hash := crc32.NewIEEE()

	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}

	checksum := hash.Sum(nil)
	return hex.EncodeToString(checksum), nil
}

func vmExists(name string) (VM, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c := exec.CommandContext(ctx, vboxBin, "list", "vms")
	out, err := c.Output()
	if err != nil {
		return nil, false
	}

	if bytes.Contains(out, []byte("\""+name+"\"")) {
		return &vm{id: name}, true
	}

	return nil, false
}
