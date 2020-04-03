package store

import (
	"context"
	"errors"
	"fmt"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/google/uuid"
	logger "github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"sync"
	"time"
)

var (
	ErrUnknownTeam        = errors.New("Unknown team")
	ErrEmailAlreadyExists = errors.New("Email is already registered")

	//ErrEmptyTag            = errors.New("Tag cannot be empty")
	ErrUnknownFlag         = errors.New("Unknown flag")
	ErrFlagAlreadyComplete = errors.New("Flag is already completed")
	ErrChallengeDuplicate  = errors.New("Challenge duplication")
)

type TeamStore interface {
	GetTeamByToken(string) (*Team, error)
	SaveTeam(*Team) error
	GetTeamByID(string) (*Team, error)
	GetTeamByEmail(string) (*Team, error)
	GetTeams() []*Team
	UpdateTeamAccessed(string, time.Time) error
	UpdateTeamSolvedChallenges(string) error
	SaveTokenForTeam(string, *Team) error
	//DeleteToken(string) error //todo might be useful to have
}

type teamstore struct {
	dbc pbc.StoreClient
	eConf EventConfig
	m sync.RWMutex
	teams  map[string]*Team
	tokens map[string]string
	emails map[string]string
	names  map[string]string
}

func NewTeamStore(conf EventConfig, dbc pbc.StoreClient) *teamstore {
	ts := &teamstore{
		dbc:	dbc,
		eConf:  conf,
		teams:  map[string]*Team{},
		tokens: map[string]string{},
		names:  map[string]string{},
		emails: map[string]string{},
	}

	return ts
}

func (es *teamstore) GetTeamByToken(token string) (*Team, error) {
	es.m.RLock()
	defer es.m.RUnlock()

	id, ok := es.tokens[token]
	if !ok {
		return &Team{}, UnknownTokenErr
	}

	t, ok := es.teams[id]
	if !ok {
		return &Team{}, UnknownTeamErr
	}

	return t, nil
}

// Save the Team on the DB and on TeamStore
func (es *teamstore) SaveTeam(t *Team) error {
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

func(es *teamstore) SaveTokenForTeam (token string, in *Team) error {
	es.m.Lock()
	defer es.m.Unlock()
	if token == "" {
		return &EmptyVarErr{Var:"Token"}
	}
	es.tokens[token]= in.ID()
	return nil
}

func (es *teamstore) GetTeamByID(id string) (*Team, error) {
	es.m.RLock()

	t, ok := es.teams[id]
	if !ok {
		es.m.RUnlock()
		return nil, ErrUnknownTeam
	}

	es.m.RUnlock()
	return t, nil
}

func (es *teamstore) GetTeamByEmail(email string) (*Team, error) {
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
func (es *teamstore) GetTeams() []*Team {
	es.m.RLock()
	teams := make([]*Team, len(es.teams))
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

// Update the Team Solved Challenges on the DB
func (es *teamstore) UpdateTeamSolvedChallenges(teamId string) error{
	es.m.RLock()

	es.m.RUnlock()
	return nil
}

type Challenge struct {
	Name    string			//challenge name
	Tag     Tag				//challenge tag
	Value   string			//challenge flag value
}

type Team struct {
	m sync.RWMutex
	id             string
	email          string
	name           string
	hashedPassword string
	challenges     map[Flag]TeamChallenge
}

type TeamChallenge struct {
	Tag         Tag
	CompletedAt *time.Time
}

func NewTeam(email, name, password, id, hashedPass string) *Team {
	var hPass []byte
	if hashedPass==""{
		hPass ,_ = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	} else {
		hPass = []byte(hashedPass)
	}

	email = strings.ToLower(email)
	if id == "" {
		id =  uuid.New().String()[0:8]
	}

	return &Team{
		id:             id,
		email:          email,
		name:           name,
		hashedPassword: string(hPass),
		challenges:     map[Flag]TeamChallenge{},
	}
}

func (t *Team) ID() string {
	t.m.RLock()
	id := t.id
	t.m.RUnlock()

	return id
}

func (t *Team) Email() string {
	t.m.RLock()
	email := t.email
	t.m.RUnlock()

	return email
}


func(t *Team) GetHashedPassword() string{
	t.m.RLock()
	defer t.m.RUnlock()
	return t.hashedPassword
}

func (t *Team) Name() string {
	t.m.RLock()
	name := t.name
	t.m.RUnlock()

	return name
}

func (t *Team) IsTeamSolvedChallenge(tag string) *time.Time {
	chals := t.challenges
	for _, chal := range chals {
		if chal.Tag == Tag(tag) {
			if chal.CompletedAt != nil {
				return chal.CompletedAt
			}
		}
	}
	return nil
}

func (t *Team) IsPasswordEqual(pass string) bool {
	t.m.RLock()
	err := bcrypt.CompareHashAndPassword([]byte(t.hashedPassword), []byte(pass))
	t.m.RUnlock()
	return err == nil
}

func (t *Team) AddChallenge(c Challenge) (Flag, error) {
	t.m.Lock()
	for _, chal := range t.challenges {
		if chal.Tag == c.Tag {
			t.m.Unlock()
			return Flag{}, ErrChallengeDuplicate
		}
	}

	f , err := NewFlagFromString(c.Value)
	if err !=nil {
		logger.Debug().Msgf("Error creating haaukins flag from given string %s", err)
		return Flag{}, err
	}
	t.challenges[f] = TeamChallenge{
		Tag: c.Tag,
	}

	t.m.Unlock()
	return f, nil
}

func (t *Team) GetChallenges(order ...Tag) []TeamChallenge {
	t.m.RLock()
	var chals []TeamChallenge
	if len(order) > 0 {
	loop:
		for _, tag := range order {
			for _, chal := range t.challenges {
				if tag == chal.Tag {
					chals = append(chals, chal)
					continue loop
				}
			}
		}
		t.m.RUnlock()
		return chals
	}

	for _, chal := range t.challenges {
		chals = append(chals, chal)
	}

	t.m.RUnlock()
	return chals
}

func (t *Team) VerifyFlag(tag Challenge, f Flag) error {
	t.m.Lock()
	chal, ok := t.challenges[f]

	if !ok {
		t.m.Unlock()
		return ErrUnknownFlag
	}

	fmt.Println(string(chal.Tag)+" ... "+string(tag.Tag))
	if chal.Tag != tag.Tag{
		t.m.Unlock()
		return ErrUnknownFlag
	}

	if chal.CompletedAt != nil {
		t.m.Unlock()
		return ErrFlagAlreadyComplete
	}
	now := time.Now()
	chal.CompletedAt = &now
	t.challenges[f] = chal

	t.m.Unlock()
	return nil
}

//func (es *Team) GetTeams() []Team {
//	var teams []Team
//	for _, t := range es.GetTeams() {
//		teams = append(teams, t)
//	}
//
//	return teams
//}
