package store

import (
	"fmt"
	"sync"

	"github.com/aau-network-security/go-ntp/exercise"
)

type UnknownExerTagErr struct {
	tag string
}

func (uee *UnknownExerTagErr) Error() string {
	return fmt.Sprintf("Unknown exercise tag: %s", uee.tag)
}

type ExerTagExistsErr struct {
	tag string
}

func (eee *ExerTagExistsErr) Error() string {
	return fmt.Sprintf("Exercise tag already exists: %s", eee.tag)
}

type exercisestore struct {
	m         sync.Mutex
	tags      map[string]*exercise.Config
	exercises []*exercise.Config
	hooks     []func([]exercise.Config) error
}

type ExerciseStore interface {
	GetExercisesByTags(string, ...string) ([]exercise.Config, error)
	CreateExercise(exercise.Config) error
	DeleteExerciseByTag(string) error
	ListExercises() []exercise.Config
}

func NewExerciseStore(exercises []exercise.Config, hooks ...func([]exercise.Config) error) (ExerciseStore, error) {
	s := exercisestore{
		tags: map[string]*exercise.Config{},
	}

	for _, e := range exercises {
		if err := s.CreateExercise(e); err != nil {
			return nil, err
		}
	}

	s.hooks = hooks

	return &s, nil
}

func (es *exercisestore) GetExercisesByTags(tag string, otherTags ...string) ([]exercise.Config, error) {
	es.m.Lock()
	defer es.m.Unlock()

	configs := make([]exercise.Config, len(otherTags)+1)
	for i, t := range append([]string{tag}, otherTags...) {
		e, ok := es.tags[t]
		if !ok {
			return nil, &UnknownExerTagErr{t}
		}

		configs[i] = *e
	}

	return configs, nil
}

func (es *exercisestore) ListExercises() []exercise.Config {
	exer := make([]exercise.Config, len(es.exercises))
	for i, e := range es.exercises {
		exer[i] = *e
	}

	return exer
}

func (es *exercisestore) CreateExercise(e exercise.Config) error {
	es.m.Lock()
	defer es.m.Unlock()

	if err := e.Validate(); err != nil {
		return err
	}

	for _, t := range e.Tags {
		if _, ok := es.tags[t]; ok {
			return &ExerTagExistsErr{t}
		}
	}

	for _, t := range e.Tags {
		es.tags[t] = &e
	}

	es.exercises = append(es.exercises, &e)

	return es.RunHooks()
}

func (es *exercisestore) DeleteExerciseByTag(t string) error {
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
