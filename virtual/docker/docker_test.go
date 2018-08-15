package docker_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	ntpdocker "github.com/aau-network-security/go-ntp/virtual/docker"
	fdocker "github.com/fsouza/go-dockerclient"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
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
		err := c.Close()
		if err != nil {
			t.Fatalf("Could not cleanup machine after test")
		}
	}
}

// tests - Create, ID, Start, Stop, Close
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
	assert.Equal(t, nil, err)
	_, notOk := err.(*fdocker.NoSuchContainer)
	assert.Equal(t, false, notOk)

	// ensure it is not running after being created
	con, err := dockerClient.InspectContainer(containerId)
	assert.Equal(t, "created", con.State.Status)

	// testing start
	err = c1.Start()
	assert.Equal(t, nil, err)
	con, err = dockerClient.InspectContainer(containerId)
	assert.Equal(t, nil, err)
	assert.Equal(t, "running", con.State.Status)
	assert.Equal(t, true, con.State.Running)

	// testing stop
	err = c1.Stop()
	assert.Equal(t, nil, err)
	con, err = dockerClient.InspectContainer(containerId)
	assert.Equal(t, nil, err)
	assert.Equal(t, "exited", con.State.Status)
	assert.Equal(t, false, con.State.Running)

	// testing kill
	err = c1.Close()
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
	var dExec *fdocker.Exec

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
	tt := []struct {
		name        string
		portBinding map[string]string
		hostIP      string
		hostPort    string
		guestPort   fdocker.Port
		err         error
	}{
		{
			name:        "no tcp",
			portBinding: map[string]string{"8080": "0.0.0.0:80"},
			hostIP:      "0.0.0.0",
			hostPort:    "80",
			guestPort:   "8080/tcp",
			err:         nil,
		}, {
			name:        "tcp specified",
			portBinding: map[string]string{"8080/tcp": "0.0.0.0:80"},
			hostIP:      "0.0.0.0",
			hostPort:    "80",
			guestPort:   "8080/tcp",
			err:         nil,
		}, {
			name:        "udp specified",
			portBinding: map[string]string{"8080/udp": "0.0.0.0:80"},
			hostIP:      "0.0.0.0",
			hostPort:    "80",
			guestPort:   "8080/udp",
			err:         nil,
		}, {
			name:        "host binding",
			portBinding: map[string]string{"8080": "127.0.0.1:80"},
			hostIP:      "127.0.0.1",
			hostPort:    "80",
			guestPort:   "8080/tcp",
			err:         nil,
		}, {
			name:        "no host binding",
			portBinding: map[string]string{"8080/tcp": "80"},
			hostIP:      "",
			hostPort:    "80",
			guestPort:   "8080/tcp",
			err:         nil,
		}, {
			name:        "invalid host binding",
			portBinding: map[string]string{"8080/tcp": "0.0.0.0:invalid:80"},
			hostIP:      "0.0.0.0",
			hostPort:    "80",
			guestPort:   "8080/tcp",
			err:         ntpdocker.InvalidHostBinding,
		}, {
			name:        "invalid protocol in host binding",
			portBinding: map[string]string{"8080": "0.0.0.0:80/tcp"},
			hostIP:      "",
			hostPort:    "80",
			guestPort:   "8080/tcp",
			err:         ntpdocker.InvalidHostBinding,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c1, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
				Image:        "alpine",
				PortBindings: tc.portBinding,
			})

			assert.Equal(t, tc.err, err)

			if c1 == nil {
				if tc.err == err {
					return
				}
				t.Fatalf("Unexpected error: %s", err)
			}

			con, err := dockerClient.InspectContainer(c1.ID())
			assert.Equal(t, nil, err)

			for guestPort, host := range con.HostConfig.PortBindings {
				assert.Equal(t, tc.guestPort.Port(), guestPort.Port())
				assert.Equal(t, tc.hostIP, host[0].HostIP)
				assert.Equal(t, tc.hostPort, host[0].HostPort)
			}

			err = c1.Close()
			assert.Equal(t, nil, err)
		})
	}
}

// test error with too low mem assigned
func TestErrorMem(t *testing.T) {
	tt := []struct {
		name     string
		memory   uint
		expected int64
		err      error
	}{
		{
			name:     "low memory",
			memory:   49,
			expected: 0,
			err:      ntpdocker.TooLowMemErr,
		}, {
			name:     "exact memory",
			memory:   50,
			expected: 50 * 1024 * 1024,
			err:      nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c1, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
				Image: "alpine",
				Resources: &ntpdocker.Resources{
					MemoryMB: tc.memory,
					CPU:      5000,
				}})

			assert.Equal(t, tc.err, err)

			if c1 == nil {
				if tc.err == err {
					return
				}
				t.Fatalf("Unexpected error: %s", err)
			}

			con, err := dockerClient.InspectContainer(c1.ID())
			assert.Equal(t, nil, err)
			assert.Equal(t, tc.expected, con.HostConfig.Memory)

			err = c1.Close()
			assert.Equal(t, nil, err)
		})
	}
}

// test error with mounting
func TestErrorMount(t *testing.T) {
	tt := []struct {
		name     string
		value    string
		expected string
		err      error
	}{
		{
			name:     "valid",
			value:    "/tmp:/myextratmp",
			expected: "/tmp:/myextratmp",
			err:      nil,
		}, {
			name:     "no mount point",
			value:    "/myextratmp",
			expected: "/myextratmp",
			err:      ntpdocker.InvalidMount,
		}, {
			name:     "too many mount points",
			value:    "/tmp:/myextratmp:/canihaveanotheroneplease",
			expected: "/myextratmp",
			err:      ntpdocker.InvalidMount,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c1, err := ntpdocker.NewContainer(ntpdocker.ContainerConfig{
				Image:  "eyjhb/backup-rotate",
				Mounts: []string{tc.value},
			})

			assert.Equal(t, tc.err, err)

			if c1 == nil {
				if tc.err == err {
					return
				}
				t.Fatalf("Unexpected error: %s", err)
			}

			con, err := dockerClient.InspectContainer(c1.ID())
			assert.Equal(t, nil, err)

			assert.Equal(t, tc.expected, con.HostConfig.Mounts[0].Source+":"+con.HostConfig.Mounts[0].Target)

			err = c1.Close()
			assert.Equal(t, nil, err)
		})
	}
}
