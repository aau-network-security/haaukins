package exercise

import (
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type Library struct {
	exercises map[string]*Config
}

type RawLibrary struct {
	Exercises []Config `yaml:"exercise"`
}

func NewLibrary(path string) (*Library, error) {
	var raw RawLibrary
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	exercises := map[string]*Config{}
	for _, e := range raw.Exercises {
		for _, t := range e.Tags {
			exercises[t] = &e
		}
	}

	return &Library{
		exercises: exercises,
	}, nil
}

func (lib *Library) GetByTags(tag string, otherTags ...string) ([]Config, error) {
	var configs []Config

	for i, t := range append([]string{tag}, otherTags...) {
		e, ok := lib.exercises[t]
		if !ok {
			return nil, UnknownTagErr
		}

		configs[i] = *e
	}

	return configs, nil
}
