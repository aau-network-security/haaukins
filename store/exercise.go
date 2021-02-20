package store

import (
	"errors"
	"fmt"

	"github.com/aau-network-security/haaukins/virtual/docker"
)

var (
	EmptyExTags         = errors.New("Exercise cannot have zero tags")
	ImageNotDefinedErr  = errors.New("image cannot be empty")
	MemoryNotDefinedErr = errors.New("memory cannot be empty")
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

func (e Exercise) Flags() []FlagConfig {
	var res []FlagConfig

	for _, conf := range e.Instance {
		res = append(res, conf.Flags...)
	}
	return res
}

func (e Exercise) Validate() error {
	if e.Tag == "" {
		return &EmptyVarErr{Var: "Tag", Type: "Exercise"}
	}

	return nil
}

type ContainerOptions struct {
	DockerConf docker.ContainerConfig
	Records    []RecordConfig
	Challenges []Challenge
}

func (e Exercise) ContainerOpts() []ContainerOptions {
	var opts []ContainerOptions

	for _, conf := range e.Instance {
		var challenges []Challenge
		envVars := make(map[string]string)

		for _, flag := range conf.Flags {
			value := flag.StaticFlag
			// static flag format in exercises file
			//  should obey flag format HKN{*********}
			if value == "" {
				// flag is not static
				value = NewFlag().String()
			}

			challenges = append(challenges, Challenge{
				Tag:   flag.Tag,
				Value: value,
			})
			envVars[flag.EnvVar] = value
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

		opts = append(opts, ContainerOptions{
			DockerConf: spec,
			Records:    conf.Records,
			Challenges: challenges,
		})
	}

	return opts
}

func (rc RecordConfig) Validate() error {
	if rc.Type == "" {
		return &EmptyVarErr{Var: "Type", Type: "Record config"}
	}

	if rc.Name == "" {
		return &EmptyVarErr{Var: "Name", Type: "Record config"}
	}

	return nil
}

func (rc RecordConfig) Format(ip string) string {
	return fmt.Sprintf("%s %s %s", rc.Name, rc.Type, ip)
}

func (fc FlagConfig) Validate() error {
	if err := fc.Tag.Validate(); err != nil {
		return err
	}

	if fc.Name == "" {
		return &EmptyVarErr{Var: "Name", Type: "Flag ServiceConfig"}
	}

	if fc.StaticFlag == "" && fc.EnvVar == "" {
		return &EmptyVarErr{Var: "Static or Env", Type: "Flag ServiceConfig"}
	}

	if fc.Points == 0 {
		return &EmptyVarErr{Var: "Points", Type: "Flag ServiceConfig"}
	}

	return nil
}

func (evc EnvVarConfig) Validate() error {
	if evc.EnvVar == "" {
		return &EmptyVarErr{Var: "Env", Type: "Environment Variable"}
	}

	if evc.Value == "" {
		return &EmptyVarErr{Var: "Value", Type: "Environment Variable"}
	}

	return nil
}

func (ic InstanceConfig) Validate() error {
	if ic.Image == "" {
		return &EmptyVarErr{Var: "Image", Type: "Instance ServiceConfig"}
	}
	if ic.MemoryMB < 0 {
		return errors.New("memory cannot be negative")
	}
	if ic.CPU < 0 {
		return errors.New("cpu cannot be negative")
	}

	return nil
}

type VboxConfig struct {
	ExerciseInstanceConfig `yaml:",inline"`
}

func (vc VboxConfig) Validate() error {
	if vc.Image == "" {
		return ImageNotDefinedErr
	}
	if vc.MemoryMB == 0 {
		return MemoryNotDefinedErr
	}
	return nil
}

//type exercisestore struct {
//	m            sync.Mutex
//	tags         map[Tag]*Exercise
//	exercises    []*Exercise
//	exerciseInfo []FlagConfig
//	hooks        []func([]Exercise) error
//}
//func (es *exercisestore) UpdateExercisesFile(path string) (ExerciseStore, error) {
//	exStore, err := NewExerciseFile(path)
//	if err != nil {
//		return nil, err
//	}
//	return exStore, nil
//}
//type ExerciseStore interface {
//	GetExercisesByTags(...Tag) ([]Exercise, error)
//	GetExercisesInfo(Tag) []FlagConfig
//	IsSecretExercise(Tag) (bool, error)
//	CreateExercise(Exercise) error
//	DeleteExerciseByTag(Tag) error
//	ListExercises() []Exercise
//	UpdateExercisesFile(string) (ExerciseStore, error)
//}
//
//func NewExerciseStore(exercises []Exercise, hooks ...func([]Exercise) error) (ExerciseStore, error) {
//	s := exercisestore{
//		tags:         map[Tag]*Exercise{},
//		exerciseInfo: []FlagConfig{},
//	}
//
//	for _, e := range exercises {
//		if err := s.CreateExercise(e); err != nil {
//			return nil, err
//		}
//	}
//
//	for _, e := range s.exercises {
//		for _, i := range e.Flags() {
//			s.exerciseInfo = append(s.exerciseInfo, i)
//		}
//	}
//
//	s.hooks = hooks
//
//	return &s, nil
//}
//
//func (es *exercisestore) IsSecretExercise(t Tag) (bool, error) {
//	es.m.Lock()
//	defer es.m.Unlock()
//	ex := es.tags[t]
//	if ex == nil {
//		return false, fmt.Errorf("No exercise with tag %s", t)
//	}
//	return ex.Secret, nil
//}
//
//func (es *exercisestore) GetExercisesInfo(tag Tag) []FlagConfig {
//	es.m.Lock()
//	defer es.m.Unlock()
//	var exer []FlagConfig
//
//	for _, e := range es.exerciseInfo {
//		if strings.Contains(string(e.Tag), string(tag)) {
//			exer = append(exer, e)
//		}
//	}
//	return exer
//}
//
//func (es *exercisestore) GetExercisesByTags(tags ...Tag) ([]Exercise, error) {
//	es.m.Lock()
//	defer es.m.Unlock()
//
//	configs := make([]Exercise, len(tags))
//	for i, t := range tags {
//		e, ok := es.tags[t]
//		if !ok {
//			return nil, &UnknownExerTagErr{t}
//		}
//
//		configs[i] = *e
//	}
//
//	return configs, nil
//}
//
//func (es *exercisestore) ListExercises() []Exercise {
//	exer := make([]Exercise, len(es.exercises))
//	for i, e := range es.exercises {
//		exer[i] = *e
//	}
//
//	return exer
//}
//
//func (es *exercisestore) CreateExercise(e Exercise) error {
//	es.m.Lock()
//	defer es.m.Unlock()
//
//	if err := e.Validate(); err != nil {
//		return err
//	}
//
//	for _, t := range e.Tags {
//		if _, ok := es.tags[t]; ok {
//			return &ExerTagExistsErr{string(t)}
//		}
//	}
//
//	for _, t := range e.Tags {
//		es.tags[t] = &e
//	}
//
//	es.exercises = append(es.exercises, &e)
//
//	return es.RunHooks()
//}
//
//func (es *exercisestore) DeleteExerciseByTag(t Tag) error {
//	es.m.Lock()
//	defer es.m.Unlock()
//
//	e, ok := es.tags[t]
//	if !ok {
//		return &UnknownExerTagErr{t}
//	}
//
//	for _, ta := range e.Tags {
//		delete(es.tags, ta)
//	}
//
//	for i, ex := range es.exercises {
//		if ex == e {
//			es.exercises = append(es.exercises[:i], es.exercises[i+1:]...)
//			break
//		}
//	}
//
//	return es.RunHooks()
//}
//
//func (es *exercisestore) RunHooks() error {
//	for _, h := range es.hooks {
//		if err := h(es.ListExercises()); err != nil {
//			return err
//		}
//	}
//
//	return nil
//}
//
//func NewExerciseFile(path string) (ExerciseStore, error) {
//	var conf struct {
//		Exercises []Exercise `yaml:"exercises"`
//	}
//
//	var m sync.Mutex
//	save := func() error {
//		m.Lock()
//		defer m.Unlock()
//
//		bytes, err := yaml.Marshal(conf)
//		if err != nil {
//			return err
//		}
//
//		return ioutil.WriteFile(path, bytes, 0644)
//	}
//
//	// file exists
//	if _, err := os.Stat(path); !os.IsNotExist(err) {
//		f, err := ioutil.ReadFile(path)
//		if err != nil {
//			return nil, err
//		}
//
//		err = yaml.Unmarshal(f, &conf)
//		if err != nil {
//			return nil, err
//		}
//
//		for _, ex := range conf.Exercises {
//			if err := ex.Validate(); err != nil {
//				return nil, err
//			}
//		}
//	}
//
//	return NewExerciseStore(conf.Exercises, func(e []Exercise) error {
//		conf.Exercises = e
//		return save()
//	})
//}
