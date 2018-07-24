package ntp

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/aau-network-security/go-ntp/svcs/dhcp"
	"github.com/aau-network-security/go-ntp/svcs/dns"
	"github.com/aau-network-security/go-ntp/virtual/docker"
)

func init() {
	rand.Seed(time.Now().Unix())
}

type Exercise struct {
	DockerConfs []dockerConfig
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

type ExerciseGroup struct {
	exercises  []*Exercise
	network    *docker.Network
	containers []docker.Container
	dnsRecords []string
	dnsServer  *dns.Server
	dnsAddr    string
}

func NewExerciseGroup(exercises []*Exercise) *ExerciseGroup {
	return &ExerciseGroup{
		exercises: exercises,
	}
}

func (eg *ExerciseGroup) startExercise(e *Exercise, updateDNS bool) error {
	for _, conf := range e.DockerSpecs() {
		conf.Spec.DNS = []string{eg.dnsAddr}
		cont, err := docker.NewContainer(conf.Spec)
		if err != nil {
			return err
		}

		if err := cont.Start(); err != nil {
			return err
		}

		ip, err := eg.network.Connect(cont)
		if err != nil {
			return err
		}

		for _, record := range conf.Records {
			eg.dnsRecords = append(eg.dnsRecords, fmt.Sprintf("%s %s", record, ip))
		}

		eg.containers = append(eg.containers, cont)
	}

	if updateDNS {
		if err := eg.updateDNS(); err != nil {
			return err
		}
	}

	return nil
}

func (eg *ExerciseGroup) updateDNS() error {
	if eg.dnsServer != nil {
		if err := eg.dnsServer.Stop(); err != nil {
			return err
		}
	}

	serv, err := dns.New(eg.dnsRecords)
	if err != nil {
		return err
	}

	if _, err := eg.network.Connect(serv.Container(), dns.PreferedIP); err != nil {
		return err
	}

	eg.dnsServer = serv
	eg.dnsAddr = eg.network.FormatIP(dns.PreferedIP)

	return nil
}

func (eg *ExerciseGroup) Start() error {
	var err error
	eg.network, err = docker.NewNetwork()
	if err != nil {
		return err
	}

	dhcp, err := dhcp.New(eg.network.FormatIP)
	if err != nil {
		return err
	}

	if _, err := eg.network.Connect(dhcp.Container(), 2); err != nil {
		return err
	}

	for _, e := range eg.exercises {
		if err := eg.startExercise(e, false); err != nil {
			return err
		}
	}

	if err := eg.updateDNS(); err != nil {
		return err
	}

	return nil
}
