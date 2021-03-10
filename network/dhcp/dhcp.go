// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package dhcp

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aau-network-security/haaukins/network/dns"
	"github.com/aau-network-security/haaukins/virtual/docker"
)

type Server struct {
	cont     docker.Container
	confFile string
	dns      string
	subnet   string
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
	maxRange := format(254)
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
		Labels: map[string]string{
			"hkn": "lab_dhcpd",
		},
	})

	return &Server{
		cont:     cont,
		confFile: confFile,
		dns:      dns,
		subnet:   subnet,
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
func (dhcp *Server) LabSubnet() string {
	return dhcp.subnet
}

func (dhcp *Server) LabDNS() string {
	return dhcp.dns
}

func (dhcp *Server) Stop() error {
	return dhcp.cont.Stop()
}
