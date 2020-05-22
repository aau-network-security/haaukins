package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/google/uuid"
	logger "github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUnknownTeam        = errors.New("Unknown team")
	ErrEmailAlreadyExists = errors.New("Email is already registered")

	//ErrEmptyTag            = errors.New("Tag cannot be empty")
	ErrUnknownFlag         = errors.New("Unknown flag")
	ErrFlagAlreadyComplete = errors.New("Flag is already completed")
	ErrChallengeDuplicate  = errors.New("Challenge duplication")
	ErrTeamAlreadyExist    = errors.New("Team is already exists")
)

type TeamStore interface {
	GetTeamByToken(string) (*Team, error)
	SaveTeam(*Team) error
	GetTeamByID(string) (*Team, error)
	GetTeamByEmail(string) (*Team, error)
	GetTeams() []*Team
	SaveTokenForTeam(string, *Team) error
}

type teamstore struct {
	dbc    pbc.StoreClient
	eConf  EventConfig
	m      sync.RWMutex
	teams  map[string]*Team
	tokens map[string]string
	emails map[string]string
	names  map[string]string
}

func NewTeamStore(conf EventConfig, dbc pbc.StoreClient) *teamstore {
	ts := &teamstore{
		dbc:    dbc,
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
		return &Team{}, fmt.Errorf("GetTeamByToken function error %v", UnknownTeamErr)
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
	username := t.Name()
	_, ok = es.names[username]
	if ok {
		es.m.Unlock()
		return ErrTeamAlreadyExist
	}
	es.names[username] = t.Name()
	es.emails[email] = t.ID()
	es.teams[t.ID()] = t

	_, err := es.dbc.AddTeam(context.Background(), &pbc.AddTeamRequest{
		Id:       strings.TrimSpace(t.ID()),
		EventTag: strings.TrimSpace(string(es.eConf.Tag)),
		Email:    strings.TrimSpace(t.Email()),
		Name:     strings.TrimSpace(t.Name()),
		Password: strings.TrimSpace(t.GetHashedPassword()),
	})
	if err != nil {
		es.m.Unlock()
		return err
	}

	t.dbc = es.dbc

	es.m.Unlock()
	return nil
}

func (es *teamstore) SaveTokenForTeam(token string, in *Team) error {
	es.m.Lock()
	defer es.m.Unlock()
	if token == "" {
		return &EmptyVarErr{Var: "Token"}
	}
	if in.ID() == "" {
		return fmt.Errorf("SaveTokenForTeam function error %v", UnknownTeamErr)
	}
	es.tokens[token] = in.ID()
	return nil
}

func (es *teamstore) GetTeamByID(id string) (*Team, error) {
	es.m.RLock()

	t, ok := es.teams[id]
	if !ok {
		es.m.RUnlock()
		return nil, fmt.Errorf("GetTeamByID function error %v", UnknownTeamErr)
	}

	es.m.RUnlock()
	return t, nil
}

func (es *teamstore) GetTeamByEmail(email string) (*Team, error) {
	es.m.RLock()

	tid, ok := es.emails[email]
	if !ok {
		es.m.RUnlock()
		return nil, fmt.Errorf("GetTeamByEmail function error %v", UnknownTeamErr)
	}
	t, ok := es.teams[tid]
	if !ok {
		es.m.RUnlock()
		return nil, fmt.Errorf("GetTeamByEmail function error %v", UnknownTeamErr)
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

type Challenge struct {
	Name  string //challenge name
	Tag   Tag    //challenge tag
	Value string //challenge flag value
}

type Team struct {
	m              sync.RWMutex
	dbc            pbc.StoreClient
	id             string
	email          string
	name           string
	hashedPassword string
	challenges     map[Flag]TeamChallenge
	solvedChalsDB  []TeamChallenge //json got from the DB containing list of solved Challenges
}

type TeamChallenge struct {
	Tag         Tag
	CompletedAt *time.Time
}

func NewTeam(email, name, password, id, hashedPass, solvedChalsDB string, dbc pbc.StoreClient) *Team {
	var hPass []byte
	if hashedPass == "" {
		hPass, _ = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	} else {
		hPass = []byte(hashedPass)
	}

	email = strings.ToLower(email)
	if id == "" {
		id = uuid.New().String()[0:8]
	}

	solvedChals, err := ParseSolvedChallenges(solvedChalsDB)
	if err != nil {
		logger.Debug().Msgf(err.Error())
	}

	return &Team{
		dbc:            dbc,
		id:             id,
		email:          strings.TrimSpace(email),
		name:           strings.TrimSpace(name),
		hashedPassword: string(hPass),
		challenges:     map[Flag]TeamChallenge{},
		solvedChalsDB:  solvedChals,
	}
}

func ParseSolvedChallenges(solvedChalsDB string) ([]TeamChallenge, error) {

	type Challenge struct {
		Tag         string `json:"tag"`
		CompletedAt string `json:"completed-at"`
	}

	var solvedChallengesDB []Challenge
	var solvedChallenges []TeamChallenge

	//in case the team is created from amigo
	if solvedChalsDB == "" {
		return solvedChallenges, nil
	}

	if err := json.Unmarshal([]byte(solvedChalsDB), &solvedChallengesDB); err != nil {
		return solvedChallenges, err
	}

	for _, chal := range solvedChallengesDB {
		completedAt, _ := time.Parse(displayTimeFormat, chal.CompletedAt)

		solvedChallenges = append(solvedChallenges, TeamChallenge{
			Tag:         Tag(chal.Tag),
			CompletedAt: &completedAt,
		})
	}
	return solvedChallenges, nil
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

	f, err := NewFlagFromString(c.Value)
	if err != nil {
		logger.Debug().Msgf("Error creating haaukins flag from given string %s", err)
		return Flag{}, err
	}

	//get the solved challenge if solved
	var solvedOne TeamChallenge
	for _, solvedChal := range t.solvedChalsDB {
		if solvedChal.Tag == c.Tag {
			solvedOne = solvedChal
		}
	}

	if solvedOne != (TeamChallenge{}) {
		t.challenges[f] = TeamChallenge{
			Tag:         c.Tag,
			CompletedAt: solvedOne.CompletedAt,
		}
	} else {
		t.challenges[f] = TeamChallenge{
			Tag: c.Tag,
		}
	}

	t.m.Unlock()
	return f, nil
}

func (t *Team) VerifyFlag(tag Challenge, f Flag) error {
	t.m.Lock()
	chal, ok := t.challenges[f]

	if !ok {
		t.m.Unlock()
		return ErrUnknownFlag
	}

	if chal.Tag != tag.Tag {
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

	err := t.UpdateTeamSolvedChallenges(chal)
	if err != nil {
		logger.Debug().Msgf("Unable to write the solved challenges in the DB")
	}

	t.m.Unlock()
	return nil
}

// Update the Team access time on the DB
func (t *Team) UpdateTeamAccessed(time time.Time) error {
	t.m.RLock()
	_, err := t.dbc.UpdateTeamLastAccess(context.Background(), &pbc.UpdateTeamLastAccessRequest{
		TeamId:   t.ID(),
		AccessAt: time.Format(displayTimeFormat),
	})

	if err != nil {
		t.m.RUnlock()
		return err
	}

	t.m.RUnlock()
	return nil
}

// Update the Team Solved Challenges on the DB
func (t *Team) UpdateTeamSolvedChallenges(chal TeamChallenge) error {

	_, err := t.dbc.UpdateTeamSolvedChallenge(context.Background(), &pbc.UpdateTeamSolvedChallengeRequest{
		TeamId:      t.id,
		Tag:         string(chal.Tag),
		CompletedAt: chal.CompletedAt.Format(displayTimeFormat),
	})

	if err != nil {
		return err
	}
	return nil
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

func (t *Team) GetHashedPassword() string {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.hashedPassword
}

func (t *Team) Name() string {
	t.m.RLock()
	defer t.m.RUnlock()
	name := t.name

	return name
}

//Not used anywhere
//func (t *Team) GetChallenges(order ...Tag) []TeamChallenge {
//	t.m.RLock()
//	var chals []TeamChallenge
//	if len(order) > 0 {
//	loop:
//		for _, tag := range order {
//			for _, chal := range t.challenges {
//				if tag == chal.Tag {
//					chals = append(chals, chal)
//					continue loop
//				}
//			}
//		}
//		t.m.RUnlock()
//		return chals
//	}
//
//	for _, chal := range t.challenges {
//		chals = append(chals, chal)
//	}
//
//	t.m.RUnlock()
//	return chals
//}
