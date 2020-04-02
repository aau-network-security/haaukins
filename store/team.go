package store

import (
	"context"
	"errors"
	"fmt"
	"github.com/aau-network-security/haaukins"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/rs/zerolog/log"
	"sync"
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

	CreateTokenForTeam(string, *haaukins.Team) error
	//DeleteToken(string) error
}

type teamstore struct {
	dbc pbc.StoreClient
	eConf EventConfig
	m sync.RWMutex			//todo do i need it if i am using the db ? i dont think so, right?
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

func (es *teamstore) SaveTeam(t *haaukins.Team) error {
	es.m.Lock()
	fmt.Println("AAAAAAAAAAAAAAAAAAAAAAA inside save team function TeamStore")
	//todo save the team in the DB


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
