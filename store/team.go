package store

import (
	"errors"
	"sync"
	"github.com/rs/zerolog/log"
	"github.com/aau-network-security/haaukins"
)

var (
	ErrUnknownTeam        = errors.New("Unknown team")
	ErrEmailAlreadyExists = errors.New("Email is already registered")
)

type TeamStore interface {
	CreateTeam(*haaukins.Team) error // todo: implement me !
	GetTeamByToken(string) (*haaukins.Team, error) // todo: implement me
	SaveTeam(*haaukins.Team) error
	GetTeamByID(string) (*haaukins.Team, error)
	GetTeamByEmail(string) (*haaukins.Team, error)
	GetTeams() []*haaukins.Team

	CreateTokenForTeam(string, *haaukins.Team) error // todo: implement me
	//DeleteToken(string) error // todo: implement me
}

type teamstore struct {
	m sync.RWMutex
	teams  map[string]*haaukins.Team
	tokens map[string]string
	emails map[string]string
	names  map[string]string
}

//func (es *teamstore) RunHooks() error {
//	teams := es.GetTeams()
//	for _, h := range es.hooks {
//		if err := h(teams); err != nil {
//			return err
//		}
//	}
//
//	return nil
//}
//
//
//type TeamStoreOpt func(ts *teamstore)
//
//
//
//func WithPostTeamHook(hook func(teams []*haaukins.Team) error) func(ts *teamstore) {
//	return func(ts *teamstore) {
//		ts.hooks = append(ts.hooks, hook)
//	}
//}

func NewTeamStore() *teamstore {
	ts := &teamstore{
		teams:  map[string]*haaukins.Team{},
		tokens: map[string]string{},
		names:  map[string]string{},
		emails: map[string]string{},
	}

	return ts
}

func (es *teamstore) CreateTeam(t *haaukins.Team) error {
	//todo edit with db conection
	es.m.Lock()
	defer es.m.Unlock()

	if _, ok := es.teams[t.ID()]; ok {
		return TeamExistsErr
	}

	es.teams[t.ID()] = t
	es.emails[t.Email()] = t.ID()
	es.names[t.Name()] = t.ID()

	return nil
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

func (es *teamstore) SaveTeam(t *haaukins.Team) error {
	es.m.Lock()

	email := t.Email()
	_, ok := es.emails[email]
	if ok {
		es.m.Unlock()
		return ErrEmailAlreadyExists
	}else{
		es.emails[t.Email()] = t.ID()
	}
	es.emails[email] = t.ID()
	es.teams[t.ID()] = t
	es.m.Unlock()

	return nil
}

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


func (es *teamstore) GetTeams() []*haaukins.Team {
	log.Debug().Msg("WITHIN GETTTEAMS FUNCTION")
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
