package exercise_test

//import (
//	"testing"
//
//	"github.com/aau-network-security/go-ntp/exercise"
//	docker "github.com/fsouza/go-dockerclient"
//	"github.com/stretchr/testify/assert"
//)
//
//func TestBasicEnvironment(t *testing.T) {
//	conf := exercise.Config{
//		Name: "Test Exercise",
//		Tags: []string{"test"},
//		DockerConfs: []exercise.DockerConfig{
//			{
//				Image: "nginx",
//			},
//		},
//	}
//
//	dclient, err := docker.NewClient("unix:///var/run/docker.sock")
//	assert.Nil(t, err, "Unable to access docker environment")
//
//	containers, err := dclient.ListContainers(docker.ListContainersOptions{})
//	assert.Nil(t, err, "Unable to list containers")
//	preContCount := len(containers)
//
//	networks, err := dclient.ListNetworks()
//	assert.Nil(t, err, "Unable to list networks")
//	preNetCount := len(networks)
//
//	env, err := exercise.NewEnvironment(conf)
//	assert.Nil(t, err, "Unable to create new environment")
//
//	err = env.Start()
//	assert.Nil(t, err, "Unexpected error while starting environment")
//
//	containers, err = dclient.ListContainers(docker.ListContainersOptions{})
//	assert.Nil(t, err, "Unable to list containers")
//	postStartContCount := len(containers)
//
//	networks, err = dclient.ListNetworks()
//	assert.Nil(t, err, "Unable to list networks")
//	postStartNetCount := len(networks)
//
//	// dhcp + dns + exercise container = 3
//	assert.Equal(t, preContCount+3, postStartContCount, "Expected three containers to be started")
//	assert.Equal(t, preNetCount+1, postStartNetCount, "Expected one docker network to be started")
//
//	err = env.Close()
//	assert.Nil(t, err, "Unable to kill environment")
//
//	containers, err = dclient.ListContainers(docker.ListContainersOptions{})
//	assert.Nil(t, err, "Unable to list containers")
//	postKillContCount := len(containers)
//
//	assert.Equal(t, postKillContCount, preContCount, "Expected no containers to be running, but some still active")
//
//	networks, err = dclient.ListNetworks()
//	assert.Nil(t, err, "Unable to list networks")
//	postKillNetCount := len(networks)
//
//	assert.Equal(t, postKillNetCount, preNetCount, "Expected no networks to be running, but some still active")
//}
