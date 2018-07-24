package dhcp

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aau-network-security/go-ntp/svcs/dns"
	"github.com/aau-network-security/go-ntp/virtual/docker"
)

type DHCP struct {
	cont     docker.Container
	confFile string
}

func New(format func(n int) string) (*DHCP, error) {
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

	cont, err := docker.NewContainer(docker.ContainerConfig{
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
	if err != nil {
		return nil, err
	}

	if err := cont.Start(); err != nil {
		return nil, err
	}

	return &DHCP{
		cont:     cont,
		confFile: confFile,
	}, nil
}

func (dhcp *DHCP) Container() docker.Container {
	return dhcp.cont
}

func (dhcp *DHCP) Stop() error {
	if err := os.Remove(dhcp.confFile); err != nil {
		return err
	}

	if err := dhcp.cont.Kill(); err != nil {
		return err
	}

	return nil
}
