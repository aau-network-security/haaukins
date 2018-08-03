package lab

import (
	"testing"
)

func TestLoadConfig(t *testing.T) {
	conf, _ := LoadConfig("test_resources/test_lab.yml")
	if len(conf.Exercises) != 2 {
		t.Fatalf("Unexpected number of exercises (expected 2): %d", len(conf.Exercises))
	}
	if conf.Exercises[0].Name != "Exercise 1" {
		t.Fatalf("Unexpected exercise name (expected 'Exercise 1'): %s", conf.Exercises[0].Name)
	}
}
