package virtual

import (
	"net"
	"strconv"
	"strings"
)

type Instance interface {
	Start() error
	Kill() error
}

type ResourceResizer interface {
	SetRAM(uint) error
	SetCPU(uint) error
}

type ID string

func GetAvailablePort() uint {
	l, _ := net.Listen("tcp", ":0")
	parts := strings.Split(l.Addr().String(), ":")
	l.Close()

	p, _ := strconv.Atoi(parts[len(parts)-1])

	return uint(p)
}
