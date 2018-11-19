package virtual

import (
	"context"
	"io"
	"net"
	"strconv"
	"strings"
)

const (
	Running = State(0)
	Stopped = State(1)
	Error   = State(2)
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
