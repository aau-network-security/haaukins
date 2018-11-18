package exercise_test

import (
	"testing"

	"time"

	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/store"
	"github.com/fsouza/go-dockerclient"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

func TestBasicEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	conf := store.Exercise{
		Name: "Test Exercise",
		Tags: []store.Tag{"test"},
		DockerConfs: []store.DockerConfig{
			{
				ExerciseInstanceConfig: store.ExerciseInstanceConfig{
					InstanceConfig: store.InstanceConfig{Image: "nginx"},
				},
			},
		},
	}

	dclient, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		t.Fatalf("unable to access docker environment: %s", err)
	}

	containers, err := dclient.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		t.Fatalf("unable to list containers: %s", err)
	}
	preContCount := len(containers)

	networks, err := dclient.ListNetworks()
	if err != nil {
		t.Fatalf("unable to list networks: %s", err)
	}
	preNetCount := len(networks)

	env, err := exercise.NewEnvironment(nil, conf)
	if err != nil {
		t.Fatalf("unable to create new environment: %s", err)
	}

	err = env.Start()
	if err != nil {
		t.Fatalf("unexpected error while starting environment: %s", err)
	}

	containers, err = dclient.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		t.Fatalf("unable to list containers: %s", err)
	}
	postStartContCount := len(containers)
	for i := 0; i < 3 && preContCount+3 != postStartContCount; i++ {
		time.Sleep(500 * time.Millisecond)

		// dhcp + dns + exercise container = 3
		containers, err = dclient.ListContainers(docker.ListContainersOptions{})
		if err != nil {
			t.Fatalf("unable to list containers: %s", err)
		}
		postStartContCount = len(containers)
	}

	if preContCount+3 != postStartContCount {
		t.Fatalf("expected three containers to be started (%d + 3 != %d)", preContCount, postStartContCount)
	}

	networks, err = dclient.ListNetworks()
	if err != nil {
		t.Fatalf("unable to list networks: %s", err)
	}
	postStartNetCount := len(networks)

	if preNetCount+1 != postStartNetCount {
		t.Fatalf("expected one docker network to be started")
	}

	err = env.Close()
	if err != nil {
		t.Fatalf("unable to kill environment: %s", err)
	}

	containers, err = dclient.ListContainers(docker.ListContainersOptions{})
	if err != nil {
		t.Fatalf("unable to list containers: %s", err)
	}
	postKillContCount := len(containers)

	if postKillContCount != preContCount {
		t.Fatalf("expected no containers to be running, but some still active")
	}

	networks, err = dclient.ListNetworks()
	if err != nil {
		t.Fatalf("unable to list networks: %s", err)
	}
	postKillNetCount := len(networks)

	if postKillNetCount != preNetCount {
		t.Fatalf("Expected no networks to be running, but some still active")
	}
}
