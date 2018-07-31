package docker_test

import (
	"fmt"
	"testing"

	docker "github.com/fsouza/go-dockerclient"
)

func TestDocker(t *testing.T) {
	DefaultClient, _ := docker.NewClient("unix:///var/run/docker.sock")

	fmt.Println(DefaultClient.PullImage(docker.PullImageOptions{
		Repository: "ubuntu",
		Tag:        "latest",
	}, docker.AuthConfiguration{}))
	// c1, err := docker.NewContainer(docker.ContainerConfig{
	// 	Image: "aau/sql-server",
	// 	EnvVars: map[string]string{
	// 		"HOST": "server",
	// 	},
	// 	Resources: &docker.Resources{
	// 		MemoryMB: 50,
	// 	},
	// })
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// defer c1.Kill()

	// c2, err := docker.NewContainer(docker.ContainerConfig{
	// 	Image: "aau/sql-client",
	// 	EnvVars: map[string]string{
	// 		"HOST": "server",
	// 	},
	// })
	// defer c2.Kill()

	// if err != nil {
	// 	t.Fatal(err)
	// }

	// err = c2.Link(c1, "server")
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// fmt.Println(c1.Start())
	// fmt.Println(c2.Start())

	// time.Sleep(29 * time.Second)
}
