package exercise

import (
	"context"

	"github.com/aau-network-security/haaukins/store"
	"github.com/rs/zerolog/log"
)

type Tracker interface {
	AttachTeam(ctx context.Context, t *store.Team) error
}

type exercisestate struct {
	tag store.Tag
	// Stores unsolved challenges
	unsolved map[store.Tag]bool
}

type exerciselist struct {
	// Converts between challenge tags and their parent exercise tag
	chalmap map[store.Tag]*exercisestate
	env     Environment
}

func (el *exerciselist) markSolved(chal store.TeamChallenge) {
	ctx := context.Background()
	state, ok := el.chalmap[chal.Tag]
	if !ok {
		log.Error().Msg("Could not find challenge")
		return
	}

	delete(state.unsolved, chal.Tag)

	if len(state.unsolved) == 0 {
		err := el.env.RemoveByTag(ctx, state.tag)
		if err != nil {
			log.Error().Err(err).Msg("Could not remove solved challenge")
			return
		}
		log.Debug().Msg("Removed challenge")
	}
}

func (el *exerciselist) AttachTeam(ctx context.Context, t *store.Team) error {
	hook := func(t *store.Team, chal store.TeamChallenge) error {
		log.Info().Msgf("solve for team %s, challenge %s", t.ID(), chal.Tag)
		go el.markSolved(chal)
		return nil
	}

	// TODO Take care of already solved challenges
	t.AttachSolvedHook(hook)

	return nil
}

// Creates a new tracker from a provider and a blank Environment to control
func NewTracker(exer []store.Exercise, env Environment) Tracker {
	el := &exerciselist{
		chalmap: map[store.Tag]*exercisestate{},
		env:     env,
	}

	// Loop all flags for all challenges
	for _, ex := range exer {
		state := &exercisestate{
			unsolved: map[store.Tag]bool{},
			tag:      ex.Tags[0],
		}

		for _, dockconf := range ex.DockerConfs {
			for _, flag := range dockconf.Flags {
				state.unsolved[flag.Tag] = true
				el.chalmap[flag.Tag] = state
			}
		}
		for _, vboxconf := range ex.VboxConfs {
			for _, flag := range vboxconf.Flags {
				state.unsolved[flag.Tag] = true
				el.chalmap[flag.Tag] = state
			}
		}
	}

	return el
}
