package docker_test

import (
	"fmt"
	"testing"

    "github.com/rs/zerolog/log"
    "github.com/stretchr/testify/assert"
    ntpdocker "github.com/aau-network-security/go-ntp/virtual/docker"
    fdocker "github.com/fsouza/go-dockerclient"
)

var dockerClient, dockerErr = fdocker.NewClient("unix:///var/run/docker.sock")

func init() {
    fmt.Println("Init function!")

    if dockerErr != nil {
        log.Fatal().Err(dockerErr)
    }
}


func testCleanup(t *testing.T, c ntpdocker.Container) func() {
    return func() {
        err := c.Kill()
        if err != nil {
            t.Fatalf("Could not cleanup machine after test")
        }
    }
}

func TestDockerContainer(t *testing.T) {
    c1, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
        Image: "alpine",
        Resources: &ntpdocker.Resources{
            MemoryMB: 50,
            CPU: 5000,
    }})
    defer testCleanup(t, c1)()

    if err != nil {
        t.Fatalf("Could not create new container: %v", err)
    }
}

func TestErrorHostBinding(t *testing.T) {
    tests := []struct{
        portBinding map[string]string
        hostIP string
        hostPort string
        guestPort fdocker.Port
        err error
    }{
        {
            portBinding: map[string]string{"8080": "0.0.0.0:80"},
            hostIP: "0.0.0.0",
            hostPort: "80",
            guestPort: "8080/tcp",
            err: nil,
        },{
            portBinding: map[string]string{"8080/tcp": "0.0.0.0:80"},
            hostIP: "0.0.0.0",
            hostPort: "80",
            guestPort: "8080/tcp",
            err: nil,
        },{
            portBinding: map[string]string{"8080/udp": "0.0.0.0:80"},
            hostIP: "0.0.0.0",
            hostPort: "80",
            guestPort: "8080/udp",
            err: nil,
        },{
            portBinding: map[string]string{"8080": "127.0.0.1:80"},
            hostIP: "127.0.0.1",
            hostPort: "80",
            guestPort: "8080/tcp",
            err: nil,
        },{
            portBinding: map[string]string{"8080/tcp": "80"},
            hostIP: "",
            hostPort: "80",
            guestPort: "8080/tcp",
            err: nil,
        },{
            portBinding: map[string]string{"8080/tcp": "0.0.0.0:invalid:80"},
            hostIP: "0.0.0.0",
            hostPort: "80",
            guestPort: "8080/tcp",
            err: ntpdocker.InvalidHostBinding,
        },{
            portBinding: map[string]string{"8080": "0.0.0.0:80/tcp"},
            hostIP: "",
            hostPort: "80",
            guestPort: "8080/tcp",
            err: ntpdocker.InvalidHostBinding,
        },
    }

    for _, test := range tests {
        c1, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
            Image: "alpine",
            PortBindings: test.portBinding,
        })

        if err != test.err {
            t.Fatalf("Did not get expected error: %s, but instead %s", test.err, err)
        }

        if c1 == nil  {
            if test.err == err {
                continue
            }
            t.Fatalf("Unexpected error: %s", err)
        }

        con, err := dockerClient.InspectContainer(c1.ID())

        if err != nil {
            t.Fatalf("Could not inspect container: %v", err)
        }

        for guestPort, host := range con.HostConfig.PortBindings {
            assert.Equal(t, test.guestPort.Port(), guestPort.Port())
            assert.Equal(t, test.hostIP, host[0].HostIP)
            assert.Equal(t, test.hostPort, host[0].HostPort)
        }

        err = c1.Kill()
        if err != nil {
            t.Fatalf("Could not destroy container after use..")
        }
    }
}

func TestErrorMem(t *testing.T) {
    tests := []struct{
        memory uint
        expected int64
        err error
    }{
        {
            memory: 49,
            expected: 0,
            err: ntpdocker.TooLowMemErr,
        },{
            memory: 50,
            expected: 50*1024*1024,
            err: nil,
        },
    }

    for _, test := range tests {
        c1, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
            Image: "alpine",
            Resources: &ntpdocker.Resources{
                MemoryMB: test.memory,
                CPU: 5000,
        }})

        if err != test.err {
            t.Fatalf("Did not get expected error: %s, but instead %s", test.err, err)
        }

        if c1 == nil  {
            if test.err == err {
                continue
            }
            t.Fatalf("Unexpected error: %s", err)
        }

        con, err := dockerClient.InspectContainer(c1.ID())

        if err != nil {
            t.Fatalf("Could not inspect container: %v", err)
        }

        assert.Equal(t, test.expected, con.HostConfig.Memory)

        err = c1.Kill()
        if err != nil {
            t.Fatalf("Could not destroy container after use..")
        }
    }
    _, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
        Image: "alpine",
        Resources: &ntpdocker.Resources{
            MemoryMB: 49,
            CPU: 5000,
        }})

    if err != ntpdocker.TooLowMemErr {
        t.Fatalf("Allowed to create machine with less than 50 MB Memory: %v", err)
    }

    c1, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
        Image: "alpine",
        Resources: &ntpdocker.Resources{
            MemoryMB: 50,
            CPU: 5000,
    }})
    defer testCleanup(t, c1)()

    if err != nil {
        t.Fatalf("Could not create machine with 50 MB Memory: %v", err)
    }
}

func TestErrorMount(t *testing.T) {
    tests := []struct{
        value string
        expected string
        err error
    }{
        {
            value: "/tmp:/myextratmp",
            expected: "/tmp:/myextratmp",
            err: nil,
        },{
            value: "/myextratmp",
            expected: "/myextratmp",
            err: ntpdocker.InvalidMount,
        },
    }

    for _, test := range tests {
        c1, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
            Image: "eyjhb/backup-rotate",
            Mounts: []string{test.value},
        })

        if err != test.err {
            t.Fatalf("Did not get expected error: %s, but instead %s", test.err, err)
        }

        if c1 == nil  {
            if test.err == err {
                continue
            }
            t.Fatalf("Unexpected error: %s", err)
        }

        con, err := dockerClient.InspectContainer(c1.ID())

        if err != nil {
            t.Fatalf("Could not inspect container: %v", err)
        }

        assert.Equal(t, test.expected, con.HostConfig.Mounts[0].Source+":"+con.HostConfig.Mounts[0].Target)

        err = c1.Kill()
        if err != nil {
            t.Fatalf("Could not destroy container after use..")
        }
    }
}
