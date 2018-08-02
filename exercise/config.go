package exercise

import (
	"fmt"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type Library interface {
	ByTag(string) *Config
}

type library struct {
	Exercises   []*Config `yaml:"exercise"`
	tagExercise map[string]*Config
}

func (lib *library) ByTag(t string) *Config {
	return lib.tagExercise[t]
}

func LoadConfig(path string) (Library, error) {
	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lib *library
	if err := yaml.Unmarshal(rawData, &lib); err != nil {
		return nil, err
	}

	lib.tagExercise = make(map[string]*Config)
	for _, e := range lib.Exercises {
		for _, t := range e.Tags {
			exer, ok := lib.tagExercise[t]
			if ok {
				return nil, fmt.Errorf("Redundant tag \"%s\" (used for: %s and %s)", t, exer.Name, e.Name)
			}
			lib.tagExercise[t] = e
		}
	}

	return lib, nil
}
