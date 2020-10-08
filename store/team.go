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
	"github.com/rs/zerolog/log"
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
	GetTeamByUsername(string) (*Team, error)
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
	es.names[username] = t.ID()
	es.emails[email] = t.ID()
	es.teams[t.ID()] = t

	_, err := es.dbc.AddTeam(context.Background(), &pbc.AddTeamRequest{
		Id:       strings.TrimSpace(t.ID()),
		EventTag: strings.TrimSpace(string(es.eConf.Tag)),
		Email:    strings.TrimSpace(t.Email()),
		Name:     strings.TrimSpace(t.Name()),
		Password: t.GetHashedPassword(),
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
func (es *teamstore) GetVPNConn(teamid string) []string {
	es.m.RLock()
	t, ok := es.teams[teamid]
	if !ok {
		es.m.RUnlock()
		return []string{}
	}
	return t.vpnConf
}

func (es *teamstore) GetTeamByUsername(username string) (*Team, error) {
	es.m.RLock()

	tid, ok := es.names[username]
	if !ok {
		es.m.RUnlock()
		return nil, fmt.Errorf("GetTeamByUsername function error %v", UnknownTeamErr)
	}
	t, ok := es.teams[tid]
	if !ok {
		es.m.RUnlock()
		return nil, fmt.Errorf("GetTeamByUsername function error %v", UnknownTeamErr)
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

type VpnConn struct {
	// client information [Interface]
	IName   string // interface
	PrivKey string // client private key (not server)
	LabDNS  string // lab dns information
	// server information [Peer]
	PubKey     string
	Endpoint   string
	AllowedIps string //lab subnet
}

type Team struct {
	m              sync.RWMutex
	dbc            pbc.StoreClient
	id             string
	email          string
	name           string
	hashedPassword string
	// used to suspend resources for that team
	// this is last access time to environment of a team.
	lastAccess    time.Time
	challenges    map[Flag]TeamChallenge
	solvedChalsDB []TeamChallenge //json got from the DB containing list of solved Challenges
	skippedChals  []string        //json got from the DB containing list of solved Challenges
	vpnKeys       map[int]string
	vpnConf       []string
	labSubnet     string
	isLabAssigned bool
	stepTracker   uint
}

type TeamChallenge struct {
	Tag         Tag
	CompletedAt *time.Time
}

func NewTeam(email, name, password, id, hashedPass, solvedChalsDB, skippedChalsDB string, stepTracker uint, dbc pbc.StoreClient) *Team {
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
		log.Debug().Msgf(err.Error())
	}

	var skippedChals []string
	if err := json.Unmarshal([]byte(skippedChalsDB), &skippedChals); err != nil {
		log.Debug().Msgf("Error parsing skipped challenges: %s", err.Error())
	}

	return &Team{
		dbc:            dbc,
		id:             id,
		email:          strings.TrimSpace(email),
		name:           strings.TrimSpace(name),
		hashedPassword: string(hPass),
		challenges:     map[Flag]TeamChallenge{},
		solvedChalsDB:  solvedChals,
		vpnKeys:        map[int]string{},
		isLabAssigned:  false,
		stepTracker:    stepTracker,
		skippedChals:   skippedChals,
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

func (t *Team) IsTeamSolvedChallenge(tag Tag) *time.Time {
	t.m.RLock()
	defer t.m.RUnlock()
	chals := t.challenges
	for _, chal := range chals {
		if chal.Tag == tag {
			if chal.CompletedAt != nil {
				return chal.CompletedAt
			}
		}
	}
	return nil
}

func (t *Team) IsTeamSkippedChallenge(tag string) bool {
	t.m.RLock()
	defer t.m.RUnlock()
	for _, chal := range t.skippedChals {
		if chal == tag {
			return true
		}
	}
	return false
}

func (t *Team) SkipChallenge(tag string, isSkip bool) error {
	t.m.Lock()
	defer t.m.Unlock()
	ii := 0
	for i, chal := range t.skippedChals {
		if chal == tag {
			ii = i
			break
		}
	}
	//mean that the challenge is not in the list
	//so i ll add the challenge to
	if !isSkip {
		t.skippedChals = append(t.skippedChals, tag)
	} else {
		// Remove the element at index ii from skippedChals.
		t.skippedChals[ii] = t.skippedChals[len(t.skippedChals)-1] // Copy last element to index ii.
		t.skippedChals[len(t.skippedChals)-1] = ""                 // Erase last element (write zero value).
		t.skippedChals = t.skippedChals[:len(t.skippedChals)-1]    // Truncate slice.
	}
	skippedChals, err := json.Marshal(t.skippedChals)
	if err != nil {
		return err
	}

	_, err = t.dbc.UpdateTeamSkippedChallenge(context.Background(), &pbc.UpdateTeamSkippedChallengeRequest{
		TeamId:       t.id,
		SkippedChals: string(skippedChals),
	})
	return err
}

// will be taken from amigo side
func (t *Team) SetVPNConn(clientConfig []string) {
	t.m.Lock()
	t.vpnConf = clientConfig
	t.m.Unlock()
}

func (t *Team) SetVPNKeys(id int, key string) {
	t.m.Lock()
	defer t.m.Unlock()

	t.vpnKeys[id] = key
}

func (t *Team) GetVPNKeys() map[int]string {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.vpnKeys
}

func (t *Team) GetVPNConn() []string {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.vpnConf
}

func (t *Team) SetLabInfo(labSubnet string) {
	t.m.Lock()
	t.labSubnet = labSubnet
	t.m.Unlock()
}

func (t *Team) GetLabInfo() string {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.labSubnet
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
		log.Debug().Msgf("Error creating haaukins flag from given string %s", err)
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
		log.Debug().Msgf("Unable to write the solved challenges in the DB")
	}

	t.m.Unlock()
	return nil
}

// Update the Team access time on the DB
func (t *Team) UpdateTeamAccessed(tm time.Time) error {
	t.m.RLock()
	_, err := t.dbc.UpdateTeamLastAccess(context.Background(), &pbc.UpdateTeamLastAccessRequest{
		TeamId:   t.ID(),
		AccessAt: tm.Format(displayTimeFormat),
	})
	t.lastAccess = tm
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

func (t *Team) LastAccessTime() time.Time {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.lastAccess
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

func (t *Team) IsLabAssigned() bool {
	t.m.RLock()
	defer t.m.RUnlock()

	return t.isLabAssigned
}

func (t *Team) CorrectedAssignedLab() {
	t.m.Lock()
	defer t.m.Unlock()
	t.isLabAssigned = true
}

func (t *Team) CurrentStep() uint {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.stepTracker
}

func (t *Team) AddStep() error {
	t.m.Lock()
	defer t.m.Unlock()
	t.stepTracker++
	_, err := t.dbc.UpdateTeamStepTracker(context.Background(), &pbc.UpdateTeamStepTrackerRequest{
		TeamId: t.id,
		Step:   int32(t.stepTracker),
	})
	if err != nil {
		return err
	}
	return nil
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
