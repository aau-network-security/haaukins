package dhcp

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aau-network-security/go-ntp/svcs/dns"
	"github.com/aau-network-security/go-ntp/virtual/docker"
)

type Server struct {
	cont     docker.Container
	confFile string
}

func New(format func(n int) string) (*Server, error) {
	f, err := ioutil.TempFile("", "dhcpd-conf")
	if err != nil {
		return nil, err
	}
	confFile := f.Name()

	subnet := format(0)
	dns := format(dns.PreferedIP)
	minRange := format(4)
	maxRange := format(29)
	broadcast := format(255)
	router := format(1)

	confStr := fmt.Sprintf(
		`option domain-name-servers %s;

	subnet %s netmask 255.255.255.0 {
		range %s %s;
		option subnet-mask 255.255.255.0;
		option broadcast-address %s;
		option routers %s;
	}`, dns, subnet, minRange, maxRange, broadcast, router)

	_, err = f.WriteString(confStr)
	if err != nil {
		return nil, err
	}

	cont := docker.NewContainer(docker.ContainerConfig{
		Image: "networkboot/dhcpd",
		Mounts: []string{
			fmt.Sprintf("%s:/data/dhcpd.conf", confFile),
		},
		DNS:       []string{dns},
		UsedPorts: []string{"67/udp"},
		Resources: &docker.Resources{
			MemoryMB: 50,
			CPU:      0.3,
		},
		Cmd: []string{"eth0"},
	})

	return &Server{
		cont:     cont,
		confFile: confFile,
	}, nil
}

func (dhcp *Server) Container() docker.Container {
	return dhcp.cont
}

func (dhcp *Server) Run(ctx context.Context) error {
	return dhcp.cont.Run(ctx)
}

func (dhcp *Server) Close() error {
	if err := os.Remove(dhcp.confFile); err != nil {
		return err
	}

	if err := dhcp.cont.Close(); err != nil {
		return err
	}

	return nil
}

func (dhcp *Server) Stop() error {
	return dhcp.cont.Stop()
}
