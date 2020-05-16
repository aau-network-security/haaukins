package exercise

import (
	"context"

	"github.com/aau-network-security/haaukins/store"
	"github.com/rs/zerolog/log"
)

type Tracker interface {
	GetExercises() []store.Exercise
	AttachTeam(ctx context.Context, t *store.Team) error
}

type exerciselist struct {
	exercises []store.Exercise
}

func (el *exerciselist) GetExercises() []store.Exercise {
	return el.exercises
}

func (el *exerciselist) AttachTeam(ctx context.Context, t *store.Team) error {
	hook := func(t *store.Team, chal store.TeamChallenge) error {
		log.Info().Msgf("solve for team %s, challenge %s", t.ID(), chal.Tag)
		return nil
	}

	// TODO Take care of already solved challenges
	t.AttachSolvedHook(hook)

	return nil
}

// Creates a new tracker from a provider and a blank Environment to control
func NewTracker(exer []store.Exercise) Tracker {
	el := &exerciselist{
		exercises: exer,
	}

	return el
}
