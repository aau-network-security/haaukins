package exercise_test

import (
    "fmt"
    "testing"

	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/aau-network-security/go-ntp/exercise"
    "github.com/stretchr/testify/assert"
)

func TestMain(t *testing.T) {
    assert.Equal(t, nil,nil)
    network, err := docker.NewNetwork()
    assert.Nil(t, err)

    conf := exercise.Config{
        Name: "Test Exercise",
        Tags: []string{"test"},
        DockerConfs: []exercise.DockerConfig{
            {
                Image: "nginx",
            },
        },
    }

    conf.Start()


    fmt.Println(network)
    fmt.Println(conf)

}

