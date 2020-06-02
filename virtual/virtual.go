// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package virtual

import (
	"context"
	"io"
	"net"
	"strconv"
	"strings"
)

const (
	Running   = State(0)
	Stopped   = State(1)
	Suspended = State(2)
	Error     = State(3)
)

type State int

type InstanceInfo struct {
	Image string
	Type  string
	Id    string
	State State
}

type Instance interface {
	Create(context.Context) error
	Start(context.Context) error
	Run(context.Context) error
	Suspend(context.Context) error
	Stop() error
	Info() InstanceInfo
	io.Closer
}

type ResourceResizer interface {
	SetRAM(uint) error
	SetCPU(uint) error
}

func GetAvailablePort() uint {
	l, _ := net.Listen("tcp", ":0")
	parts := strings.Split(l.Addr().String(), ":")
	l.Close()

	p, _ := strconv.Atoi(parts[len(parts)-1])

	return uint(p)
}
