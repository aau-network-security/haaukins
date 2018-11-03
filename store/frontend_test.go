package store

import (
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
