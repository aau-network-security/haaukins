package ntp

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/aau-network-security/go-ntp/virtual/docker"
)

func init() {
	rand.Seed(time.Now().Unix())
}

type ContainerSpecWithDNS struct {
	Spec    docker.ContainerConfig
	Records []string
}

func (e *Exercise) DockerSpecs() []ContainerSpecWithDNS {
	envVars := make(map[string]string)

	// Dynamic Flags
	// for _, f := range conf.Flags {
	// 	envVars[f] = uuid.New().String()
	// }

	var specs []ContainerSpecWithDNS

	for _, conf := range e.DockerConfs {
		spec := docker.ContainerConfig{
			Image:   conf.Image,
			EnvVars: envVars,
			Resources: &docker.Resources{
				MemoryMB: conf.MemoryMB,
				CPU:      conf.CPU,
			},
		}

		var records []string
		for _, r := range conf.Records {
			records = append(records, fmt.Sprintf("%s %s", r.Name, r.Type))
		}

		specs = append(specs, ContainerSpecWithDNS{
			Spec:    spec,
			Records: records,
		})
	}

	return specs
}
