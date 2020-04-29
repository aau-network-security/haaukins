// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package dns

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"io"

	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/rs/zerolog/log"
)

const (
	PreferedIP      = 3
	coreFileContent = `. {
    file zonefile
    prometheus     # enable metrics
    errors         # show errors
    log            # enable query logs
}
`
	zonePrefixContent = `$ORIGIN .
@   3600 IN SOA sns.dns.icann.org. noc.dns.icann.org. (
                2017042745 ; serial
                7200       ; refresh (2 hours)
                3600       ; retry (1 hour)
                1209600    ; expire (2 weeks)
                3600       ; minimum (1 hour)
                )

`
)

type Server struct {
	cont     docker.Container
	confFile string
	io.Closer
}

type RR struct {
	Name  string
	Type  string
	RData string
}

func (rr *RR) Format() string {
	return fmt.Sprintf("%s IN %s %s", rr.Name, rr.Type, rr.RData)
}

func New(records []RR) (*Server, error) {
	f, err := ioutil.TempFile("", "zonefile")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	c, err := ioutil.TempFile("", "Corefile")
	if err != nil {
		return nil, err
	}
	defer c.Close()

	confFile := f.Name()

	f.Write([]byte(zonePrefixContent))

	for _, r := range records {
		_, err = f.Write([]byte(r.Format() + "\n"))
		if err != nil {
			return nil, err
		}
	}

	coreFile := c.Name()

	c.Write([]byte(coreFileContent))

	f.Sync()
	cont := docker.NewContainer(docker.ContainerConfig{
		Image: "coredns/coredns:1.6.1",
		Mounts: []string{
			fmt.Sprintf("%s:/Corefile", coreFile),
			fmt.Sprintf("%s:/zonefile", confFile),
		},
		UsedPorts: []string{
			"53/tcp",
			"53/udp",
		},
		Resources: &docker.Resources{
			MemoryMB: 50,
			CPU:      0.3,
		},
		Cmd: []string{"--conf", "Corefile"},
		Labels: map[string]string{
			"hkn": "lab_dns",
		},
	})

	return &Server{
		cont:     cont,
		confFile: confFile,
	}, nil
}

func (s *Server) Container() docker.Container {
	return s.cont
}

func (s *Server) Run(ctx context.Context) error {
	return s.cont.Run(ctx)
}

func (s *Server) Close() error {
	if err := os.Remove(s.confFile); err != nil {
		log.Warn().Msgf("error while removing DNS configuration file: %s", err)
	}

	if err := s.cont.Close(); err != nil {
		log.Warn().Msgf("error while closing DNS container: %s", err)
	}

	return nil
}

func (s *Server) Stop() error {
	return s.cont.Stop()
}
