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

func New(records []string) (*Server, error) {
	f, err := ioutil.TempFile("", "unbound-conf")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	confFile := f.Name()

	for _, r := range records {
		line := fmt.Sprintf(`local-data: "%s"`, r)
		_, err = f.Write([]byte(line + "\n"))
		if err != nil {
			return nil, err
		}
	}

	f.Sync()

	cont, err := docker.NewContainer(docker.ContainerConfig{
		Image: "tpanum/unbound",
		Mounts: []string{
			fmt.Sprintf("%s:/opt/unbound/etc/unbound/a-records.conf", confFile),
		},
		UsedPorts: []string{"53/tcp"},
		Resources: &docker.Resources{
			MemoryMB: 50,
			CPU:      0.3,
		},
	})
	if err != nil {
		return nil, err
	}

	if err := cont.Start(); err != nil {
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

func (s *Server) Stop() error {
	if err := os.Remove(s.confFile); err != nil {
		return err
	}

	if err := s.cont.Kill(); err != nil {
		return err
	}

	return nil
}
