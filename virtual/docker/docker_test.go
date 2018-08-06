package docker_test

import (
	"fmt"
	"testing"
    "bytes"
    "strings"

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


// tests - Create, ID, Start, Stop, Kill
func TestContainerBase(t *testing.T) {
    // testing create
    c1, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
        Image: "alpine",
    })

    assert.Equal(t, nil, err)

    // testing ID
    containerId := c1.ID()

    // Container created
    _, err = dockerClient.InspectContainer(containerId)
    _, notOk := err.(*fdocker.NoSuchContainer)
    assert.Equal(t, false, notOk)

    // testing start
    err = c1.Start()
    assert.Equal(t, nil, err)

    // testing stop 
    err = c1.Stop()
    assert.Equal(t, nil, err)

    // testing kill 
    err = c1.Kill()
    assert.Equal(t, nil, err)

    // inspecting to see if it actully killed it
    _, err = dockerClient.InspectContainer(containerId)
    _, notOk = err.(*fdocker.NoSuchContainer)
    assert.Equal(t, true, notOk)
}

func TestLink(t *testing.T) {
    t.Skip()
    c1, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
        Image: "alpine",
    })
    assert.Equal(t, nil, err)
    defer testCleanup(t, c1)()
    c2, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
        Image: "alpine",
    })
    assert.Equal(t, nil, err)
    defer testCleanup(t, c2)()

    err = c1.Start()
    assert.Equal(t, nil, err)

    //exec, err := dockerClient.CreateExec(fdocker.CreateExecOptions{
    //    Container: c1.ID(),
    //    Cmd: []string{"ls /"},
    //    AttachStdin: true,
    //    AttachStdout: true,
    //})
    //assert.Equal(t, nil, err)
    //fmt.Println(exec)


    //details, err := dockerClient.InspectExec(exec.ID)
    //assert.Equal(t, nil, err)
    //fmt.Println(details)
    var dExec  *fdocker.Exec

    de := fdocker.CreateExecOptions{
        AttachStderr: true,
        AttachStdin:  true,
        AttachStdout: true,
        Tty:          false,
        Cmd:          []string{"ls", "/"},
        Container:    "9aa026aafb46",
    }
    if dExec, err = dockerClient.CreateExec(de); err != nil {
        fmt.Println(err)
        return
    }
    var stdout, stderr bytes.Buffer
    var reader = strings.NewReader("ls /")
    execId := dExec.ID
    opts := fdocker.StartExecOptions{
        OutputStream: &stdout,
        ErrorStream:  &stderr,
        InputStream:  reader,
        RawTerminal:  true,
    }
    if err = dockerClient.StartExec(execId, opts); err != nil {
        fmt.Println(err)
        return
    }
    fmt.Println("stdout: ", stdout)
    //<-opts.Success
    fmt.Println(stdout.String())

}



// test error with host binding 
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

        assert.Equal(t, test.err, err)

        if c1 == nil  {
            if test.err == err {
                continue
            }
            t.Fatalf("Unexpected error: %s", err)
        }

        con, err := dockerClient.InspectContainer(c1.ID())
        assert.Equal(t, nil, err)

        for guestPort, host := range con.HostConfig.PortBindings {
            assert.Equal(t, test.guestPort.Port(), guestPort.Port())
            assert.Equal(t, test.hostIP, host[0].HostIP)
            assert.Equal(t, test.hostPort, host[0].HostPort)
        }

        err = c1.Kill()
        assert.Equal(t, nil, err)
    }
}

// test error with too low mem assigned
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

        assert.Equal(t, test.err, err)

        if c1 == nil  {
            if test.err == err {
                continue
            }
            t.Fatalf("Unexpected error: %s", err)
        }

        con, err := dockerClient.InspectContainer(c1.ID())
        assert.Equal(t, nil, err)
        assert.Equal(t, test.expected, con.HostConfig.Memory)

        err = c1.Kill()
        assert.Equal(t, nil, err)
    }
}

// test error with mounting
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
        },{
            value: "/tmp:/myextratmp:/canihaveanotheroneplease",
            expected: "/myextratmp",
            err: ntpdocker.InvalidMount,
        },
    }

    for _, test := range tests {
        c1, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
            Image: "eyjhb/backup-rotate",
            Mounts: []string{test.value},
        })

        assert.Equal(t, test.err, err)

        if c1 == nil  {
            if test.err == err {
                continue
            }
            t.Fatalf("Unexpected error: %s", err)
        }

        con, err := dockerClient.InspectContainer(c1.ID())
        assert.Equal(t, nil, err)

        assert.Equal(t, test.expected, con.HostConfig.Mounts[0].Source+":"+con.HostConfig.Mounts[0].Target)

        err = c1.Kill()
        assert.Equal(t, nil, err)
    }
}
