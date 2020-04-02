package store

import (
	"context"
	"errors"
	"github.com/aau-network-security/haaukins"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"sync"
	"time"
)

var (
	ErrUnknownTeam        = errors.New("Unknown team")
	ErrEmailAlreadyExists = errors.New("Email is already registered")
)

type TeamStore interface {
	GetTeamByToken(string) (*haaukins.Team, error)
	SaveTeam(*haaukins.Team) error
	GetTeamByID(string) (*haaukins.Team, error)
	GetTeamByEmail(string) (*haaukins.Team, error)
	GetTeams() []*haaukins.Team
	UpdateTeamAccessed(string, time.Time) error
	UpdateTeamSolvedChallenges(string) error
	CreateTokenForTeam(string, *haaukins.Team) error
	//DeleteToken(string) error //todo might be useful to have
}

type teamstore struct {
	dbc pbc.StoreClient
	eConf EventConfig
	m sync.RWMutex
	teams  map[string]*haaukins.Team
	tokens map[string]string
	emails map[string]string
	names  map[string]string
}

func NewTeamStore(conf EventConfig, dbc pbc.StoreClient) *teamstore {
	ts := &teamstore{
		dbc:	dbc,
		eConf:  conf,
		teams:  map[string]*haaukins.Team{},
		tokens: map[string]string{},
		names:  map[string]string{},
		emails: map[string]string{},
	}

	return ts
}

func (es *teamstore) GetTeamByToken(token string) (*haaukins.Team, error) {
	es.m.RLock()
	defer es.m.RUnlock()

	id, ok := es.tokens[token]
	if !ok {
		return &haaukins.Team{}, UnknownTokenErr
	}

	t, ok := es.teams[id]
	if !ok {
		return &haaukins.Team{}, UnknownTeamErr
	}

	return t, nil
}

// Save the Team on the DB and on TeamStore
func (es *teamstore) SaveTeam(t *haaukins.Team) error {
	es.m.Lock()

	email := t.Email()
	_, ok := es.emails[email]
	if ok {
		es.m.Unlock()
		return ErrEmailAlreadyExists
	}
	_, err := es.dbc.AddTeam(context.Background(), &pbc.AddTeamRequest{
		Id:                   t.ID(),
		EventTag:             string(es.eConf.Tag),
		Email:                t.Email(),
		Name:                 t.Name(),
		Password:             t.GetHashedPassword(),
	})
	if err != nil {
		es.m.Unlock()
		return err
	}

	es.emails[email] = t.ID()
	es.teams[t.ID()] = t
	es.m.Unlock()
	return nil
}

//todo does it create a token or it save it in teamstore? Ahmet. I thinnk it saves it. so i can change hte name of this function
func(es *teamstore) CreateTokenForTeam (token string, in *haaukins.Team) error {
	es.m.Lock()
	defer es.m.Unlock()
	if token == "" {
		return &EmptyVarErr{Var:"Token"}
	}
	es.tokens[token]= in.ID()
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

	tid, ok := es.emails[email]
	if !ok {
		es.m.RUnlock()
		return nil, ErrUnknownTeam
	}
	t, ok := es.teams[tid]
	if !ok {
		es.m.RUnlock()
		return nil, ErrUnknownTeam
	}
	es.m.RUnlock()
	return t, nil
}

// Get the teams from the TeamStore
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

// Update the Team access time on the DB
func (es *teamstore) UpdateTeamAccessed(teamId string, time time.Time) error{
	es.m.RLock()

	_, err := es.dbc.UpdateTeamLastAccess(context.Background(), &pbc.UpdateTeamLastAccessRequest{
		TeamId:               teamId,
		AccessAt:             time.Format(displayTimeFormat),
	})

	if err != nil {
		es.m.RUnlock()
		return err
	}

	es.m.RUnlock()
	return nil
}

// Update the Team access time on the DB
func (es *teamstore) UpdateTeamSolvedChallenges(teamId string) error{
	es.m.RLock()

	es.m.RUnlock()
	return nil
}
