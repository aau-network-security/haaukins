// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package exercise

import (
	"context"

	"github.com/aau-network-security/haaukins/virtual/docker"
)

//
//func dconfFromRecords(records []store.RecordConfig) store.DockerConfig {
//	return store.DockerConfig{
//		ExerciseInstanceConfig: store.ExerciseInstanceConfig{
//			Records: records,
//		},
//	}
//}
//
//func TestContainerOpts(t *testing.T) {
//	records := []store.RecordConfig{
//		{
//			Name:  "example.org",
//			Type:  "MX",
//			RData: "mx.example.org",
//		},
//		{
//			Name:  "mx.example.org",
//			Type:  "A",
//			RData: "",
//		},
//	}
//	dockerConfs := []store.ContainerOptions{dconfFromRecords(records)}
//	conf := store.Exercise{
//		Instance: dockerConfs,
//	}
//	containerOptions := conf.ContainerOpts()
//	if len(containerOptions) != 1 {
//		t.Fatalf("Expected 1 configs, but got %d", len(containerOptions))
//	}
//	if len(containerOptions[0].Records) != 2 {
//		t.Fatalf("Expected 2 records, but got %d", len(containerOptions[0].Records))
//	}
//}

type testDockerHost struct {
	DockerHost
}

func (tdh testDockerHost) CreateContainer(ctx context.Context, conf docker.ContainerConfig) (docker.Container, error) {
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

//func TestExerciseCreate(t *testing.T) {
//	firstRecords := []store.RecordConfig{
//		{
//			Name: "example.org",
//			Type: "A",
//		},
//		{
//			Name:  "example.org",
//			Type:  "MX",
//			RData: "10 mx.example.org",
//		},
//	}
//	secondRecords := []store.RecordConfig{
//		{
//			Name:  "mx.example.org",
//			Type:  "A",
//			RData: "",
//		},
//	}
//	dockerConfs := []store.DockerConfig{
//		dconfFromRecords(firstRecords),
//		dconfFromRecords(secondRecords),
//	}
//	conf := store.Exercise{
//		DockerConfs: dockerConfs,
//	}
//	e := NewExercise(conf, testDockerHost{}, nil, &testNetwork{}, "")
//	if err := e.Create(context.Background()); err != nil {
//		t.Fatalf("Unexpected error: %v", err)
//	}
//
//	if len(e.dnsRecords) != 3 {
//		t.Fatalf("Expected 3 DNS records, but got %d", len(e.dnsRecords))
//	}
//	if e.dnsRecords[0].RData != "1.2.3.4" {
//		t.Fatalf("Expected rData '1.2.3.4', but got '%s'", e.dnsRecords[0].RData)
//	}
//}
