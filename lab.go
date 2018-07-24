package ntp

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	"github.com/aau-network-security/go-ntp/docker"
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
	dnsServer  *DNS
}

func NewExerciseGroup(exercises []*Exercise) *ExerciseGroup {
	return &ExerciseGroup{
		exercises: exercises,
	}
}

func (eg *ExerciseGroup) startExercise(e *Exercise, updateDNS bool) error {
	for _, conf := range e.DockerSpecs() {
		conf.Spec.DNS = []string{"192.168.0.3"}
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

	dns, err := NewDNS(eg.dnsRecords)
	if err != nil {
		return err
	}

	if _, err := eg.network.Connect(dns.cont, 3); err != nil {
		return err
	}

	eg.dnsServer = dns

	return nil
}

func (eg *ExerciseGroup) Start() error {
	var err error
	eg.network, err = docker.NewNetwork()
	if err != nil {
		return err
	}

	dhcp, err := NewDHCP(eg.network.FormatIP)
	if err != nil {
		return err
	}

	if _, err := eg.network.Connect(dhcp.cont, 2); err != nil {
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

type DNS struct {
	cont     docker.Container
	confFile string
}

func NewDNS(records []string) (*DNS, error) {
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
		Image: "mvance/unbound",
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

	return &DNS{
		cont:     cont,
		confFile: confFile,
	}, nil

}

func (dns *DNS) Stop() error {
	if err := os.Remove(dns.confFile); err != nil {
		return err
	}

	if err := dns.cont.Kill(); err != nil {
		return err
	}

	return nil
}

type DHCP struct {
	cont     docker.Container
	confFile string
}

func NewDHCP(format func(n int) string) (*DHCP, error) {
	f, err := ioutil.TempFile("", "dhcpd-conf")
	if err != nil {
		return nil, err
	}
	confFile := f.Name()

	subnet := format(0)
	dns := format(3)
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

func (dhcp *DHCP) Stop() error {
	if err := os.Remove(dhcp.confFile); err != nil {
		return err
	}

	if err := dhcp.cont.Kill(); err != nil {
		return err
	}

	return nil
}
