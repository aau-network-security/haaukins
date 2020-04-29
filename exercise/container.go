package exercise

import (
	"github.com/aau-network-security/haaukins/store"
)

type Container interface {
	GetExercises() []store.Exercise
	// SubmitFlag
	// AttachHook
}

type exerciselist struct {
	exercises []store.Exercise
}

func (el *exerciselist) GetExercises() []store.Exercise {
	return el.exercises
}

func NewContainerFromProvider(elib store.ExerciseStore, ep store.ExerciseProvider) (Container, error) {
	var (
		err error
		el  exerciselist
	)

	el.exercises, err = elib.GetExercisesByTags(ep.GetExerciseTags()...)
	if err != nil {
		return nil, err
	}

	return &el, nil
}
