// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

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
	"strconv"
	"strings"
	"sync"
	"time"

	"math"
	"regexp"

	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/virtual"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	stateRegex = `State:\s*(.*)`
	nicRegex  = "\\bNIC\\b"

	vboxBin          = "VBoxManage"
	vboxModVM        = "modifyvm"
	vboxStartVM      = "startvm"
	vboxCtrlVM       = "controlvm"
	vboxUnregisterVM = "unregistervm"
	vboxShowVMInfo   = "showvminfo"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

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
	LinkedClone(context.Context, string, ...VMOpt) (VM, error)
}

type Library interface {
	GetCopy(context.Context, store.InstanceConfig, ...VMOpt) (VM, error)
	IsAvailable(string) bool
}

type vBoxLibrary struct {
	m     sync.Mutex
	pwd   string
	known map[string]VM
	locks map[string]*sync.Mutex
}

// VM information is stored in a struct
type vm struct {
	id      string
	path    string
	image   string
	opts    []VMOpt
	running bool
}

func NewVMWithSum(path, image string, checksum string, vmOpts ...VMOpt) VM {
	return &vm{
		path:  path,
		image: image,
		opts:  vmOpts,
		id:    fmt.Sprintf("%s{%s}", image, checksum),
	}
}

// Creating VM
func (vm *vm) Create(ctx context.Context) error {
	_, err := VBoxCmdContext(ctx, "import", vm.path, "--vsys", "0", "--vmname", vm.id)
	if err != nil {
		return err
	}

	for _, opt := range vm.opts {
		if err := opt(ctx, vm); err != nil {
			return err
		}
	}

	return nil
}

// when Run is called, it calls Create function within it.
func (vm *vm) Run(ctx context.Context) error {
	if err := vm.Create(ctx); err != nil {
		return err
	}

	return vm.Start(ctx)
}

func (vm *vm) Start(ctx context.Context) error {
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
	_, err := VBoxCmdContext(context.Background(), vboxCtrlVM, vm.id, "poweroff")
	if err != nil {
		log.Error().Msgf("Error while shutting down VM %s", err)
		return err
	}

	vm.running = false

	log.Debug().
		Str("ID", vm.id).
		Msg("Stopped VM")

	return nil
}

// Will call savestate on vm
func (vm *vm) Suspend(ctx context.Context) error {
	_, err := VBoxCmdContext(ctx, vboxCtrlVM, vm.id, "savestate")
	if err != nil {
		log.Error().
			Str("ID", vm.id).
			Msgf("Failed to suspend VM: %s", err)
		return err
	}

	log.Debug().
		Str("ID", vm.id).
		Msgf("Suspended vm")

	return nil
}

func (vm *vm) Close() error {
	_, err := vm.ensureStopped(nil)
	if err != nil {
		log.Warn().
			Str("ID", vm.id).
			Msgf("Failed to stop VM: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = VBoxCmdContext(ctx, vboxUnregisterVM, vm.id, "--delete")
	if err != nil {
		return err
	}

	log.Debug().
		Str("ID", vm.id).
		Msg("Closed VM")

	return nil
}

type VMOpt func(context.Context, *vm) error

func removeAllNICs(ctx context.Context, vm *vm) error {
	result, err := VBoxCmdContext(ctx, vboxShowVMInfo, vm.id)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(nicRegex)
	numberOfNICs := re.FindAll(result, -1)
	for i := 1; i <= len(numberOfNICs); i++ {
		_, err = VBoxCmdContext(ctx, vboxModVM, vm.id, "--nic"+strconv.Itoa(i), "none")
		if err != nil {
			return err
		}
	}
	return nil
}

func SetBridge(nic string) VMOpt {
	return func(ctx context.Context, vm *vm) error {
		// Removes all NIC cards from importing VMs
		if err := removeAllNICs(ctx, vm); err != nil {
			return err
		}
		// enables specified NIC card in purpose
		_, err := VBoxCmdContext(ctx, vboxModVM, vm.id, "--nic1", "bridged", "--bridgeadapter1", nic)
		if err != nil {
			return err
		}
		// allows promiscuous mode
		_, err = VBoxCmdContext(ctx, vboxModVM, vm.id, "--nicpromisc1", "allow-all")
		if err != nil {
			return err
		}

		return nil
	}
}

func SetLocalRDP(ip string, port uint) VMOpt {
	return func(ctx context.Context, vm *vm) error {
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

		_, err = VBoxCmdContext(ctx, vboxModVM, vm.id, "--clipboard", "bidirectional")
		if err != nil {
			return err
		}

		return nil
	}
}

func SetCPU(cores uint) VMOpt {
	return func(ctx context.Context, vm *vm) error {
		_, err := VBoxCmdContext(ctx, vboxModVM, vm.id, "--cpus", fmt.Sprintf("%d", cores))
		return err
	}
}

func SetRAM(mb uint) VMOpt {
	return func(ctx context.Context, vm *vm) error {
		_, err := VBoxCmdContext(ctx, vboxModVM, vm.id, "--memory", fmt.Sprintf("%d", mb))
		return err
	}
}

func (vm *vm) ensureStopped(ctx context.Context) (func(), error) {
	wasRunning := vm.running
	if vm.running {
		if err := vm.Stop(); err != nil {
			return nil, err
		}
	}

	return func() {
		if wasRunning {
			vm.Start(ctx)
		}
	}, nil
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

func (v *vm) LinkedClone(ctx context.Context, snapshot string, vmOpts ...VMOpt) (VM, error) {
	newID := strings.Replace(uuid.New().String(), "-", "", -1)
	_, err := VBoxCmdContext(ctx, "clonevm", v.id, "--snapshot", snapshot, "--options", "link", "--name", newID, "--register")
	if err != nil {
		return nil, err
	}

	vm := &vm{
		image: v.image,
		id:    newID,
	}
	for _, opt := range vmOpts {
		if err := opt(ctx, vm); err != nil {
			return nil, err
		}
	}

	return vm, nil
}

func (v *vm) state() virtual.State {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	raw, err := VBoxCmdContext(ctx, vboxShowVMInfo, v.id)
	if err != nil {
		return virtual.Error
	}

	r := regexp.MustCompile(stateRegex)
	matched := r.FindSubmatch(raw)
	if len(matched) == 0 {
		return virtual.Error
	}
	if strings.Contains(string(matched[0]), "running") {
		return virtual.Running
	}
	if strings.Contains(string(matched[0]), "saved") {
		return virtual.Suspended
	}

	return virtual.Stopped
}

func (v *vm) Info() virtual.InstanceInfo {
	return virtual.InstanceInfo{
		Image: v.image,
		Type:  "vbox",
		Id:    v.id,
		State: v.state(),
	}
}

func NewLibrary(pwd string) Library {
	return &vBoxLibrary{
		pwd:   pwd,
		known: make(map[string]VM),
		locks: make(map[string]*sync.Mutex),
	}
}

func (lib *vBoxLibrary) getPathFromFile(file string) string {
	if !strings.HasPrefix(file, lib.pwd) {
		file = filepath.Join(lib.pwd, file)
	}

	if !strings.HasSuffix(file, ".ova") {
		file += ".ova"
	}

	return file
}

func (lib *vBoxLibrary) GetCopy(ctx context.Context, conf store.InstanceConfig, vmOpts ...VMOpt) (VM, error) {
	path := lib.getPathFromFile(conf.Image)

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
		return vm.LinkedClone(ctx, "origin", vmOpts...) // if ok==true then VM will be linked without the ram value which is exist on configuration file
		// vbox.SetRAM(conf.memoryMB) on addFrontend function in lab.go fixes the problem...
	}
	// if ok==false, then following codes will be run, in that case there will be no problem because at the end instance returns with specified VMOpts parameter.
	sum, err := checksumOfFile(path)
	if err != nil {
		return nil, err
	}

	n := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	vm, ok = VmExists(n, sum)
	if !ok {
		vm = NewVMWithSum(path, n, sum)
		if err := vm.Create(ctx); err != nil {
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

	if conf.CPU != 0 {
		vmOpts = append(vmOpts, SetCPU(uint(math.Ceil(conf.CPU))))
	}

	if conf.MemoryMB != 0 {
		vmOpts = append(vmOpts, SetRAM(conf.MemoryMB))
	}

	instance, err := vm.LinkedClone(ctx, "origin", vmOpts...)
	if err != nil {
		return nil, err
	}

	return instance, nil
}

func (lib *vBoxLibrary) IsAvailable(file string) bool {
	path := lib.getPathFromFile(file)
	if _, err := os.Stat(path); err == nil {
		return true
	}

	return false
}

func checksumOfFile(filepath string) (string, error) {
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

func VmExists(image string, checksum string) (VM, bool) {
	name := fmt.Sprintf("%s{%s}", image, checksum)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := VBoxCmdContext(ctx, "list", "vms")
	if err != nil {
		return nil, false
	}

	if bytes.Contains(out, []byte("\""+name+"\"")) {
		return &vm{
			image: image,
			id:    name,
		}, true
	}

	return nil, false
}

//
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
