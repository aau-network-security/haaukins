package exercise

import (
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"testing"
)

func TestContainerOpts(t *testing.T) {
	records := []RecordConfig{
		{
			Name:  "example.org",
			Type:  "MX",
			RData: "mx.example.org",
		},
		{
			Name:  "mx.example.org",
			Type:  "A",
			RData: "",
		},
	}
	dockerConfs := []DockerConfig{
		{Records: records, Image: "aau/test-image"},
	}
	conf := Config{
		DockerConfs: dockerConfs,
	}
	containerConfigs, recordConfigs := conf.ContainerOpts()
	if len(containerConfigs) != 1 || len(recordConfigs) != 1 {
		t.Fatalf("Expected 1 configs, but got %d", len(containerConfigs))
	}
	if len(recordConfigs[0]) != 2 {
		t.Fatalf("Expected 2 records, but got %d", len(recordConfigs[0]))
	}
}

type testDockerHost struct {
	DockerHost
}

func (tdh testDockerHost) CreateContainer(conf docker.ContainerConfig) (docker.Container, error) {
	return testContainer{}, nil
}

type testContainer struct {
	docker.Container
}

type testNetwork struct {
	docker.Network
}

func (tn testNetwork) Connect(c docker.Container, ip ...int) (int, error) {
	return 1, nil
}

func (tn testNetwork) FormatIP(num int) string {
	return "1.2.3.4"
}

func TestExerciseCreate(t *testing.T) {
	firstRecords := []RecordConfig{
		{
			Name: "example.org",
			Type: "A",
		},
		{
			Name:  "example.org",
			Type:  "MX",
			RData: "10 mx.example.org",
		},
	}
	secondRecords := []RecordConfig{
		{
			Name:  "mx.example.org",
			Type:  "A",
			RData: "",
		},
	}
	dockerConfs := []DockerConfig{
		{Records: firstRecords, Image: "aau/test-image"},
		{Records: secondRecords, Image: "aau/test-image"},
	}
	conf := Config{
		DockerConfs: dockerConfs,
	}
	e := exercise{
		conf:       &conf,
		dockerHost: testDockerHost{},
		net:        &testNetwork{},
	}
	if err := e.Create(); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(e.dnsRecords) != 3 {
		t.Fatalf("Expected 3 DNS records, but got %d", len(e.dnsRecords))
	}
	if e.dnsRecords[0].RData != "1.2.3.4" {
		t.Fatalf("Expected rData '1.2.3.4', but got '%s'", e.dnsRecords[0].RData)
	}
}
