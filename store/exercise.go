package store

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/aau-network-security/go-ntp/virtual/docker"
	yaml "gopkg.in/yaml.v2"
)

var (
	EmptyExTags = errors.New("Exercise cannot have zero tags")
)

type UnknownExerTagErr struct {
	tag Tag
}

func (uee *UnknownExerTagErr) Error() string {
	return fmt.Sprintf("Unknown exercise tag: %s", uee.tag)
}

type ExerTagExistsErr struct {
	tag string
}

func (eee *ExerTagExistsErr) Error() string {
	return fmt.Sprintf("Tag already exists: %s", eee.tag)
}

type Exercise struct {
	Name        string         `yaml:"name"`
	Tags        []Tag          `yaml:"tags"`
	DockerConfs []DockerConfig `yaml:"docker"`
}

func (e Exercise) Validate() error {
	if len(e.Tags) == 0 {
		return EmptyExTags
	}

	for _, t := range e.Tags {
		if err := t.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (e Exercise) Flags() []FlagConfig {
	var res []FlagConfig

	for _, dockerConf := range e.DockerConfs {
		res = append(res, dockerConf.Flags...)
	}
	return res
}

func (e Exercise) ContainerOpts() ([]docker.ContainerConfig, [][]RecordConfig) {
	var contSpecs []docker.ContainerConfig
	var contRecords [][]RecordConfig

	for _, conf := range e.DockerConfs {
		envVars := make(map[string]string)

		for _, flag := range conf.Flags {
			envVars[flag.EnvVar] = flag.Default
		}

		for _, env := range conf.Envs {
			envVars[env.EnvVar] = env.Value
		}

		// docker config
		spec := docker.ContainerConfig{
			Image: conf.Image,
			Resources: &docker.Resources{
				MemoryMB: conf.MemoryMB,
				CPU:      conf.CPU,
			},
			EnvVars: envVars,
		}

		contSpecs = append(contSpecs, spec)
		contRecords = append(contRecords, conf.Records)
	}

	return contSpecs, contRecords
}

type Flag struct {
}

type RecordConfig struct {
	Type string `yaml:"record"`
	Name string `yaml:"name"`
}

func (rc RecordConfig) Format(ip string) string {
	return fmt.Sprintf("%s %s %s", rc.Name, rc.Type, ip)
}

type FlagConfig struct {
	Name    string `yaml:"name"`
	EnvVar  string `yaml:"env"`
	Default string `yaml:"default"`
	Points  uint   `yaml:"points"`
}

type EnvVarConfig struct {
	EnvVar string `yaml:"env"`
	Value  string `yaml:"value"`
}

type DockerConfig struct {
	Image    string         `yaml:"image"`
	Flags    []FlagConfig   `yaml:"flag"`
	Envs     []EnvVarConfig `yaml:"env"`
	Records  []RecordConfig `yaml:"dns"`
	MemoryMB uint           `yaml:"memoryMB"`
	CPU      float64        `yaml:"cpu"`
}

type exercisestore struct {
	m         sync.Mutex
	tags      map[Tag]*Exercise
	exercises []*Exercise
	hooks     []func([]Exercise) error
}

type ExerciseStore interface {
	GetExercisesByTags(...Tag) ([]Exercise, error)
	CreateExercise(Exercise) error
	DeleteExerciseByTag(Tag) error
	ListExercises() []Exercise
}

func NewExerciseStore(exercises []Exercise, hooks ...func([]Exercise) error) (ExerciseStore, error) {
	s := exercisestore{
		tags: map[Tag]*Exercise{},
	}

	for _, e := range exercises {
		if err := s.CreateExercise(e); err != nil {
			return nil, err
		}
	}

	s.hooks = hooks

	return &s, nil
}

func (es *exercisestore) GetExercisesByTags(tags ...Tag) ([]Exercise, error) {
	es.m.Lock()
	defer es.m.Unlock()

	configs := make([]Exercise, len(tags))
	for i, t := range tags {
		e, ok := es.tags[t]
		if !ok {
			return nil, &UnknownExerTagErr{t}
		}

		configs[i] = *e
	}

	return configs, nil
}

func (es *exercisestore) ListExercises() []Exercise {
	exer := make([]Exercise, len(es.exercises))
	for i, e := range es.exercises {
		exer[i] = *e
	}

	return exer
}

func (es *exercisestore) CreateExercise(e Exercise) error {
	es.m.Lock()
	defer es.m.Unlock()

	if err := e.Validate(); err != nil {
		return err
	}

	for _, t := range e.Tags {
		if _, ok := es.tags[t]; ok {
			return &ExerTagExistsErr{string(t)}
		}
	}

	for _, t := range e.Tags {
		es.tags[t] = &e
	}

	es.exercises = append(es.exercises, &e)

	return es.RunHooks()
}

func (es *exercisestore) DeleteExerciseByTag(t Tag) error {
	es.m.Lock()
	defer es.m.Unlock()

	e, ok := es.tags[t]
	if !ok {
		return &UnknownExerTagErr{t}
	}

	for _, ta := range e.Tags {
		delete(es.tags, ta)
	}

	for i, ex := range es.exercises {
		if ex == e {
			es.exercises = append(es.exercises[:i], es.exercises[i+1:]...)
			break
		}
	}

	return es.RunHooks()
}

func (es *exercisestore) RunHooks() error {
	for _, h := range es.hooks {
		if err := h(es.ListExercises()); err != nil {
			return err
		}
	}

	return nil
}

func NewExerciseFile(path string) (ExerciseStore, error) {
	var conf struct {
		Exercises []Exercise `yaml:"exercises"`
	}

	var m sync.Mutex
	save := func() error {
		m.Lock()
		defer m.Unlock()

		bytes, err := yaml.Marshal(conf)
		if err != nil {
			return err
		}

		return ioutil.WriteFile(path, bytes, 0644)
	}

	// file exists
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		f, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}

		err = yaml.Unmarshal(f, &conf)
		if err != nil {
			return nil, err
		}
	}

	return NewExerciseStore(conf.Exercises, func(e []Exercise) error {
		conf.Exercises = e
		return save()
	})
}
