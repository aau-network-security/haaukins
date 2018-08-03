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

	if len(conf.Flags()) != 3 {
		t.Fatalf("Unexpected amount of flags (expected 3): %d", len(conf.Flags()))
	}
	if conf.Flags()[0].Name != "First flag" ||
		conf.Flags()[0].EnvVar != "FLAG_1" ||
		conf.Flags()[0].Default != "flag_default_1" ||
		conf.Flags()[0].Points != 10 {
		t.Fatalf("Unexpected fields of first flag: %+v", conf.Flags()[0])
	}
}
