package store

import (
	"errors"
	"sync"

	"github.com/aau-network-security/haaukins"
)

var (
	ErrUnknownTeam        = errors.New("Unknown team")
	ErrEmailAlreadyExists = errors.New("Email is already registered")
)

type TeamStore interface {
	SaveTeam(*haaukins.Team) error
	GetTeamByID(string) (*haaukins.Team, error)
	GetTeamByEmail(string) (*haaukins.Team, error)
	GetTeams() []*haaukins.Team
}

type teamstore struct {
	m      sync.RWMutex
	teams  map[string]*haaukins.Team
	emails map[string]*haaukins.Team
}

func NewTeamStore(ts ...*haaukins.Team) *teamstore {
	teams := map[string]*haaukins.Team{}
	emails := map[string]*haaukins.Team{}
	for _, t := range ts {
		teams[t.ID()] = t
		emails[t.Email()] = t
	}

	return &teamstore{teams: teams, emails: emails}
}

func (es *teamstore) SaveTeam(t *haaukins.Team) error {
	es.m.Lock()

	email := t.Email()
	if _, ok := es.emails[email]; ok {
		es.m.Unlock()
		return ErrEmailAlreadyExists
	}

	es.teams[t.ID()] = t
	es.emails[email] = t

	es.m.Unlock()

	return nil
}

func (es *teamstore) GetTeamByID(id string) (*haaukins.Team, error) {
	es.m.RLock()

	t, ok := es.teams[id]
	if !ok {
		es.m.RUnlock()
		return nil, ErrUnknownTeam
	}

	es.m.RUnlock()
	return t, nil
}

func (es *teamstore) GetTeamByEmail(email string) (*haaukins.Team, error) {
	es.m.RLock()

	t, ok := es.emails[email]
	if !ok {
		es.m.RUnlock()
		return nil, ErrUnknownTeam
	}

	es.m.RUnlock()
	return t, nil
}

func (es *teamstore) GetTeams() []*haaukins.Team {
	es.m.RLock()
	teams := make([]*haaukins.Team, len(es.teams))
	var i int
	for _, t := range es.teams {
		teams[i] = t
		i += 1
	}
	es.m.RUnlock()

	return teams
}
