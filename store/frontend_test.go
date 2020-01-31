// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package store

import (
	tst "github.com/aau-network-security/haaukins/testing"
	"reflect"
	"testing"
)

func TestGetFrontends(t *testing.T) {
	tt := []struct {
		name              string
		existingFrontends map[string]InstanceConfig
		input             []string
		expected          []InstanceConfig
	}{
		{
			name:  "Unknown frontend",
			input: []string{"f1"},
			expected: []InstanceConfig{
				{Image: "f1", MemoryMB: 0, CPU: 0},
			},
		},
		{
			name: "Existing frontend",
			existingFrontends: map[string]InstanceConfig{
				"f1": {Image: "f1", MemoryMB: 1, CPU: 0},
			},
			input: []string{"f1"},
			expected: []InstanceConfig{
				{Image: "f1", MemoryMB: 1, CPU: 0},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			fs := frontendstore{
				frontends: tc.existingFrontends,
			}

			output := fs.GetFrontends(tc.input...)
			if !reflect.DeepEqual(output, tc.expected) {
				t.Fatalf("Expected frontends %+v, but got %+v", tc.expected, output)
			}
		})
	}
}

func TestSetMemoryMBAndCpu(t *testing.T) {
	tt := []struct {
		name              string
		existingFrontends map[string]InstanceConfig
	}{
		{
			name:              "New frontend",
			existingFrontends: map[string]InstanceConfig{},
		},
		{
			name: "Overwrite existing",
			existingFrontends: map[string]InstanceConfig{
				"f1": {Image: "f1", MemoryMB: 1, CPU: 1},
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			fs := frontendstore{
				frontends: tc.existingFrontends,
			}

			fs.SetMemoryMB("f1", 2000)
			fs.SetCpu("f1", 2)

			if actualMemory := fs.frontends["f1"].MemoryMB; actualMemory != 2000 {
				t.Fatalf("Expected 'f1' to have configured 2000 MB of memory, but has %d", actualMemory)
			}
			if actualCpu := fs.frontends["f1"].CPU; actualCpu != 2 {
				t.Fatalf("Expected 'f1' to have configured 2 CPUs, but has %f", actualCpu)
			}
		})
	}
}

func TestNewFrontendsFile(t *testing.T) {
	// since there is no dummy yml file on git repo, skipping test on Travis can be reasonable...
	tst.SkipCI(t)
	definedMemory := uint(4096)
	definedCPU := 2.0
	frontendstore, err := NewFrontendsFile("test_ymls/frontend_test.yml")
	if err != nil {
		t.Fatalf("Error: %s", err)
	}
	kaliImageInfo := frontendstore.GetFrontends("kali")
	for _, instanceInfo := range kaliImageInfo {
		if instanceInfo.MemoryMB != definedMemory {
			t.Fatalf("Defined memory does not match with the memory resides on config file")
		}
		if instanceInfo.CPU != definedCPU {
			t.Fatalf("Number of CPU does not match with the defined CPU amount")
		}

	}
}
