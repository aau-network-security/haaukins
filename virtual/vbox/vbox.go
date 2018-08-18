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

	"github.com/aau-network-security/go-ntp/virtual"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const (
	vboxBin     = "VBoxManage"
	vboxModVM   = "modifyvm"
	vboxStartVM = "startvm"
	vboxCtrlVM  = "controlvm"
)

type VBoxErr struct {
	Action string
	Output []byte
}

func (err *VBoxErr) Error() string {
	return fmt.Sprintf("VBoxError [%s]: %s", err.Action, string(err.Output))
}

type VM interface {
	virtual.Instance
	Snapshot(string) error
	LinkedClone(string, ...VMOpt) (VM, error)
}

type vm struct {
	id      string
	running bool
}

func NewVMFromOVA(path, name string, vmOpts ...VMOpt) (VM, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	_, err := VBoxCmdContext(ctx, "import", path, "--vsys", "0", "--vmname", name)
	if err != nil {
		return nil, err
	}

	vm := &vm{id: name}
	for _, opt := range vmOpts {
		if err := opt(vm); err != nil {
			return nil, err
		}
	}

	return vm, nil
}

func (vm *vm) Start() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := VBoxCmdContext(ctx, vboxStartVM, vm.id, "--type", "headless")
	if err != nil {
		return err
	}

	vm.running = true

	log.Debug().
		Str("ID", vm.id).
		Msg("Started VM")

	return nil
}

func (vm *vm) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := VBoxCmdContext(ctx, vboxCtrlVM, vm.id, "poweroff")
	if err != nil {
		return err
	}

	vm.running = false

	log.Debug().
		Str("ID", vm.id).
		Msg("Stopped VM")

	return nil
}

func (vm *vm) Close() error {
	_, err := vm.ensureStopped()
	if err != nil {
		return err
	}

	// remove vm
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = VBoxCmdContext(ctx, "unregistervm", vm.id, "--delete")
	if err != nil {
		return err
	}

	log.Debug().
		Str("ID", vm.id).
		Msg("Closed VM")

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

type VMOpt func(*vm) error

func SetBridge(nic string) VMOpt {
    return func(vm *vm) error {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        _, err := VBoxCmdContext(ctx, vboxModVM, vm.id, "--nic1", "bridged", "--bridgeadapter1", nic)
        if err != nil {
            return err
        }

        _, err = VBoxCmdContext(ctx, vboxModVM, vm.id, "--nicpromisc1", "allow-all")
        if err != nil {
            return err
        }

        return nil
    }
}

func SetLocalRDP(ip string, port uint) VMOpt {
	return func(vm *vm) error {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		_, err := VBoxCmdContext(ctx, vboxModVM, vm.id, "--vrde", "on")
		if err != nil {
			return err
		}

		_, err = VBoxCmdContext(ctx, vboxModVM, vm.id, "--vrdeproperty", fmt.Sprintf("TCP/Address=%s", ip))
		if err != nil {
			return err
		}

		_, err = VBoxCmdContext(ctx, vboxModVM, vm.id, "--vrdeproperty", fmt.Sprintf("TCP/Ports=%d", port))
		if err != nil {
			return err
		}

		_, err = VBoxCmdContext(ctx, vboxModVM, vm.id, "--vrdeauthtype", "null")
		if err != nil {
			return err
		}

		_, err = VBoxCmdContext(ctx, vboxModVM, vm.id, "--vram", "128")
		if err != nil {
			return err
		}

		return nil
	}
}

func (vm *vm) SetRAM(mb uint) error {
	start, err := vm.ensureStopped()
	if err != nil {
		return err
	}
	defer start()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = VBoxCmdContext(ctx, vboxModVM, vm.id, "--memory", fmt.Sprintf("%d", mb))
	if err != nil {
		return err
	}

	return nil
}

func (vm *vm) ensureStopped() (func(), error) {
	wasRunning := vm.running
	if vm.running {
		if err := vm.Stop(); err != nil {
			return nil, err
		}
	}

	return func() {
		if wasRunning {
			vm.Start()
		}
	}, nil
}

func (vm *vm) SetCPU(cores uint) error {
	start, err := vm.ensureStopped()
	if err != nil {
		return err
	}
	defer start()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = VBoxCmdContext(ctx, vboxModVM, vm.id, "--cpus", fmt.Sprintf("%d", cores))
	if err != nil {
		return err
	}

	return nil
}

func (vm *vm) Snapshot(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := VBoxCmdContext(ctx, "snapshot", vm.id, "take", name)
	if err != nil {
		return err
	}

	return nil
}

func (v *vm) LinkedClone(snapshot string, vmOpts ...VMOpt) (VM, error) {
	newID := strings.Replace(uuid.New().String(), "-", "", -1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := VBoxCmdContext(ctx, "clonevm", v.id, "--snapshot", snapshot, "--options", "link", "--name", newID, "--register")
	if err != nil {
		return nil, err
	}

	vm := &vm{id: newID}
	for _, opt := range vmOpts {
		if err := opt(vm); err != nil {
			return nil, err
		}
	}

	return vm, nil
}

type Library interface {
	GetCopy(string, ...VMOpt) (VM, error)
}

type vBoxLibrary struct {
	m     sync.Mutex
	pwd   string
	known map[string]VM
	locks map[string]*sync.Mutex
}

func NewLibrary(pwd string) Library {
	return &vBoxLibrary{
		pwd:   pwd,
		known: make(map[string]VM),
		locks: make(map[string]*sync.Mutex),
	}
}

func (lib *vBoxLibrary) GetCopy(path string, vmOpts ...VMOpt) (VM, error) {
	path = filepath.Join(lib.pwd, path)

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
		return vm.LinkedClone("origin", vmOpts...)
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

	return vm.LinkedClone("origin", vmOpts...)
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

	out, err := VBoxCmdContext(ctx, "list", "vms")
	if err != nil {
		return nil, false
	}

	if bytes.Contains(out, []byte("\""+name+"\"")) {
		return &vm{id: name}, true
	}

	return nil, false
}

func VBoxCmdContext(ctx context.Context, cmd string, cmds ...string) ([]byte, error) {
	command := append([]string{cmd}, cmds...)

	c := exec.CommandContext(ctx, vboxBin, command...)
	out, err := c.CombinedOutput()
	if err != nil {
		return nil, &VBoxErr{
			Action: strings.Join(command, " "),
			Output: out,
		}
	}

	return out, nil
}
