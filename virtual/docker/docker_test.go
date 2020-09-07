// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

// +build linux

package docker_test

import (
	"context"
	"fmt"
	"testing"

	hkndocker "github.com/aau-network-security/haaukins/virtual/docker"
	fdocker "github.com/fsouza/go-dockerclient"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var dockerClient, dockerErr = fdocker.NewClient("unix:///var/run/docker.sock")

func init() {
	if dockerErr != nil {
		log.Fatal().Err(dockerErr).Msg("")
	}

	zerolog.SetGlobalLevel(zerolog.Disabled)
}

func testCleanup(t *testing.T, c hkndocker.Container) func() {
	return func() {
		err := c.Close()
		if err != nil {
			t.Fatalf("Could not cleanup machine after test")
		}
	}
}

func TestContainerBase(t *testing.T) {
	// testing create. Do a long sleep to keep it alive
	c1 := hkndocker.NewContainer(hkndocker.ContainerConfig{
		Cmd:   []string{"sleep", "1d"},
		Image: "alpine",
	})
	if err := c1.Create(nil); err != nil {
		t.Fatalf("unexpected error when creating container")
	}

	containerId := c1.ID()
	inspecCon, err := dockerClient.InspectContainer(containerId)
	if err != nil {
		t.Fatalf("unable to inspect created container")
	}

	if inspecCon.State.Status != "created" {
		t.Fatalf("expected container to have status created")
	}

	err = c1.Start(nil)
	if err != nil {
		t.Fatalf("unable to start container")
	}

	inspecCon, err = dockerClient.InspectContainer(containerId)
	if err != nil {
		t.Fatalf("unable to inspect running container")
	}

	if inspecCon.State.Status != "running" {
		t.Fatalf("expected container to have status running")
	}

	err = c1.Suspend(nil)
	if err != nil {
		t.Fatalf("unable to suspend container: %s", err)
	}

	inspecCon, err = dockerClient.InspectContainer(containerId)
	if err != nil {
		t.Fatalf("unable to inspect suspended container")
	}

	if inspecCon.State.Status != "paused" {
		t.Fatalf("expected container to have status paused")
	}

	err = c1.Start(nil)
	if err != nil {
		t.Fatalf("unable to start container after suspend %s", err)
	}

	err = c1.Stop()
	if err != nil {
		t.Fatalf("unable to stop container")
	}

	inspecCon, err = dockerClient.InspectContainer(containerId)
	if err != nil {
		t.Fatalf("unable to inspect stopped container")
	}

	if inspecCon.State.Status != "exited" {
		t.Fatalf("expected container to have status exited")
	}

	err = c1.Close()
	if err != nil {
		t.Fatalf("unable to close stopped container")
	}

	_, err = dockerClient.InspectContainer(containerId)
	if _, ok := err.(*fdocker.NoSuchContainer); !ok {
		t.Fatalf("expected container to be removed")
	}
}

func TestContainerContext(t *testing.T) {
	c1 := hkndocker.NewContainer(hkndocker.ContainerConfig{
		Image: "alpine",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := c1.Create(ctx); err == nil {
		t.Fatalf("expected error when creating container with canceled context")
	}

	containerId := c1.ID()
	if containerId != "" {
		t.Fatalf("expected container id to be empty")
	}
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
			err:         hkndocker.InvalidHostBindingErr,
		}, {
			name:        "invalid protocol in host binding",
			portBinding: map[string]string{"8080": "0.0.0.0:80/tcp"},
			hostIP:      "",
			hostPort:    "80",
			guestPort:   "8080/tcp",
			err:         hkndocker.InvalidHostBindingErr,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c1 := hkndocker.NewContainer(hkndocker.ContainerConfig{
				Image:        "alpine",
				PortBindings: tc.portBinding,
			})
			if err := c1.Create(nil); err != nil {
				if tc.err == err {
					return
				}

				t.Fatalf("expected error (%v), but received: %s", tc.err, err)
			}
			defer c1.Close()

			if tc.err != nil {
				t.Fatalf("expected error (%v), but received none", tc.err)
			}

			con, err := dockerClient.InspectContainer(c1.ID())
			if err != nil {
				t.Fatalf("expected no error when inspecting container")
			}

			for guestPort, host := range con.HostConfig.PortBindings {
				if tc.guestPort.Port() != guestPort.Port() {
					t.Errorf("unexpected guest port (expected: %s): %s", tc.guestPort.Port(), guestPort.Port())
				}

				if tc.hostIP != host[0].HostIP {
					t.Errorf("unexpected host ip (expected: %s): %s", tc.hostIP, host[0].HostIP)
				}

				if tc.hostPort != host[0].HostPort {
					t.Errorf("unexpected host port (expected: %s): %s", tc.hostPort, host[0].HostPort)
				}
			}
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
			err:      hkndocker.TooLowMemErr,
		}, {
			name:     "exact memory",
			memory:   50,
			expected: 50 * 1024 * 1024,
			err:      nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c1 := hkndocker.NewContainer(hkndocker.ContainerConfig{
				Image: "alpine",
				Resources: &hkndocker.Resources{
					MemoryMB: tc.memory,
					CPU:      5000,
				}})

			if err := c1.Create(nil); err != nil {
				if tc.err == err {
					return
				}

				t.Fatalf("expected error (%v), but received: %s", tc.err, err)
			}
			defer c1.Close()

			if tc.err != nil {
				t.Fatalf("expected error (%v), but received none", tc.err)
			}

			con, err := dockerClient.InspectContainer(c1.ID())
			if err != nil {
				t.Fatalf("expected no error when inspecting container")
			}

			if m := con.HostConfig.Memory; tc.expected != m {
				t.Fatalf("unexpected amount of memory (expected: %d): %d", tc.expected, m)
			}
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
			err:      hkndocker.InvalidMountErr,
		}, {
			name:     "too many mount points",
			value:    "/tmp:/myextratmp:/canihaveanotheroneplease",
			expected: "/myextratmp",
			err:      hkndocker.InvalidMountErr,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c1 := hkndocker.NewContainer(hkndocker.ContainerConfig{
				Image:  "eyjhb/backup-rotate",
				Mounts: []string{tc.value},
			})
			if err := c1.Create(nil); err != nil {
				if tc.err == err {
					return
				}

				t.Fatalf("expected error (%v), but received: %s", tc.err, err)
			}
			defer c1.Close()

			if tc.err != nil {
				t.Fatalf("expected error (%v), but received none", tc.err)
			}

			con, err := dockerClient.InspectContainer(c1.ID())
			if err != nil {
				t.Fatalf("expected no error when inspecting container")
			}

			src := con.HostConfig.Mounts[0].Source
			trg := con.HostConfig.Mounts[0].Target
			combined := fmt.Sprintf("%s:%s", src, trg)

			if tc.expected != combined {
				t.Fatalf("unexpected mount (expected: %s): %s", tc.expected, combined)
			}
		})
	}
}
