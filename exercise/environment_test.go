package exercise_test

import (
	"testing"

	"github.com/aau-network-security/go-ntp/exercise"
	docker "github.com/fsouza/go-dockerclient"
)

func TestBasicEnvironment(t *testing.T) {
	conf := exercise.Config{
		Name: "Test Exercise",
		Tags: []string{"test"},
		DockerConfs: []exercise.DockerConfig{
			{
				Image: "nginx",
			},
		},
	}

	dclient, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		t.Fatalf("Unable to access docker environment: %s", err)
	}

	containers, err := dclient.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		t.Fatalf("Unable to list containers: %s", err)
	}
	preContCount := len(containers)

	networks, err := dclient.ListNetworks()
	if err != nil {
		t.Fatalf("Unable to list networks: %s", err)
	}
	preNetCount := len(networks)

	env, err := exercise.NewEnvironment(conf)
	if err != nil {
		t.Fatalf("Unable to create new environment: %s", err)
	}

	containers, err = dclient.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		t.Fatalf("Unable to list containers: %s", err)
	}
	postStartContCount := len(containers)

	networks, err = dclient.ListNetworks()
	if err != nil {
		t.Fatalf("Unable to list networks: %s", err)
	}
	postStartNetCount := len(networks)

	// dhcp + dns + exercise container = 3
	if preContCount+3 != postStartContCount {
		t.Fatalf("Expected three containers to be started, but %d was started", postStartContCount-preContCount)
	}

	if preNetCount+1 != postStartNetCount {
		t.Fatalf("Expected one docker network to be started, but %d was started", postStartNetCount-preNetCount)
	}

	err = env.Kill()
	if err != nil {
		t.Fatalf("Unable to kill environment: %s", err)
	}

	containers, err = dclient.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		t.Fatalf("Unable to list containers: %s", err)
	}
	postKillContCount := len(containers)

	if postKillContCount != preContCount {
		t.Fatalf("Expected no containers to be running, but %d is still active", postKillContCount-preContCount)
	}

	networks, err = dclient.ListNetworks()
	if err != nil {
		t.Fatalf("Unable to list networks: %s", err)
	}
	postKillNetCount := len(networks)

	if postKillNetCount != preNetCount {
		t.Fatalf("Expected no networks to be running, but %d is still active", postKillNetCount-preNetCount)
	}

}
