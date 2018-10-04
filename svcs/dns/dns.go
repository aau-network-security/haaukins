package dns

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aau-network-security/go-ntp/virtual/docker"
)

const (
	PreferedIP = 3
)

type Server struct {
	cont     docker.Container
	confFile string
}

type RR struct {
	Name  string
	Type  string
	RData string
}

func (rr *RR) Format() string {
	if rr.Type == "MX" {
		return fmt.Sprintf("%s IN MX 10 %s", rr.Name, rr.RData)
	}
	return fmt.Sprintf("%s IN %s %s", rr.Name, rr.Type, rr.RData)
}

func New(records []RR) (*Server, error) {
	f, err := ioutil.TempFile("", "zonefile")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	confFile := f.Name()

	zonePrefix, err := ioutil.ReadFile("zonefile-prefix")
	if err != nil {
		return nil, err
	}
	f.Write(zonePrefix)

	for _, r := range records {
		line := fmt.Sprintf(`%s`, r.Format())
		_, err = f.Write([]byte(line + "\n"))
		if err != nil {
			return nil, err
		}
	}

	f.Sync()

	cont, err := docker.NewContainer(docker.ContainerConfig{
		Image: "coredns/coredns",
		Mounts: []string{
			"Corefile:Corefile",
			fmt.Sprintf("%s:zonefile", confFile),
		},
		UsedPorts: []string{"53/tcp"},
		Resources: &docker.Resources{
			MemoryMB: 50,
			CPU:      0.3,
		},
		Cmd: []string{"--conf", "Corefile"},
	})
	if err != nil {
		return nil, err
	}

	return &Server{
		cont:     cont,
		confFile: confFile,
	}, nil

}

func (s *Server) Container() docker.Container {
	return s.cont
}

func (s *Server) Start() error {
	return s.cont.Start()
}

func (s *Server) Close() error {
	if err := os.Remove(s.confFile); err != nil {
		return err
	}

	if err := s.cont.Close(); err != nil {
		return err
	}

	return nil
}

func (s *Server) Stop() error {
	return s.cont.Close()
}
