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
	DeleteTeam(string, string) error
	GetTeamByID(string) (*Team, error)
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
	names  map[string]string
}

func NewTeamStore(conf EventConfig, dbc pbc.StoreClient) *teamstore {
	ts := &teamstore{
		dbc:    dbc,
		eConf:  conf,
		teams:  map[string]*Team{},
		tokens: map[string]string{},
		names:  map[string]string{},
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

	username := t.Name()
	_, ok := es.names[username]
	if ok {
		es.m.Unlock()
		return ErrTeamAlreadyExist
	}
	es.names[username] = t.ID()
	es.teams[t.ID()] = t

	_, err := es.dbc.AddTeam(context.Background(), &pbc.AddTeamRequest{
		Id:       strings.TrimSpace(t.ID()),
		EventTag: strings.TrimSpace(string(es.eConf.Tag)),
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

func (es *teamstore) DeleteTeam(tId, eventTag string) error {
	es.m.Lock()
	defer es.m.Unlock()
	team, ok := es.teams[tId]
	_, oki := es.names[team.Name()]
	if oki {
		delete(es.names, team.Name())
	}
	if ok {
		delete(es.teams, tId)
	}

	resp, err := es.dbc.DeleteTeam(context.TODO(), &pbc.DelTeamRequest{
		EvTag:  eventTag,
		TeamId: tId,
	})
	if err != nil {
		return err
	}
	log.Debug().Msgf("%s", resp.Message)
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
	Hosts      string
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
	lastAccess         time.Time
	challenges         map[string]TeamChallenge
	solvedChalsDB      []TeamChallenge //json got from the DB containing list of solved Challenges
	vpnKeys            map[int]string
	vpnConf            []string
	labSubnet          string
	isLabAssigned      bool
	hostsInfo          []string
	disabledChallenges map[string][]string // list of disabled children challenge tags to be used for amigo frontend
	allChallenges      map[string][]string
}

type TeamChallenge struct {
	Tag         Tag
	CompletedAt *time.Time
}

func NewTeam(email, name, password, id, hashedPass, solvedChalsDB string,
	lastAccessedT time.Time, disabledExs, allChallenges map[string][]string, dbc pbc.StoreClient) *Team {
	disabledChals := CopyMap(disabledExs)
	allChals := CopyMap(allChallenges)
	var hPass []byte
	if hashedPass == "" {
		hPass, _ = bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	} else {
		hPass = []byte(hashedPass)
	}

	if id == "" {
		id = uuid.New().String()[0:8]
	}

	solvedChals, err := ParseSolvedChallenges(solvedChalsDB)
	if err != nil {
		log.Debug().Msgf(err.Error())
	}

	return &Team{
		dbc:                dbc,
		id:                 id,
		email:              strings.TrimSpace(email),
		name:               strings.TrimSpace(name),
		hashedPassword:     string(hPass),
		challenges:         map[string]TeamChallenge{},
		solvedChalsDB:      solvedChals,
		lastAccess:         lastAccessedT,
		vpnKeys:            map[int]string{},
		isLabAssigned:      false,
		disabledChallenges: disabledChals,
		allChallenges:      allChals,
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
	t.m.Lock()
	defer t.m.Unlock()
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

func (t *Team) GetDisabledChals() []string {
	t.m.Lock()
	defer t.m.Unlock()
	var chals []string
	for _, v := range t.disabledChallenges {
		chals = append(chals, v...)
	}
	return chals
}

func (t *Team) GetChildChallenges(parentTag string) []string {
	ch, ok := t.allChallenges[parentTag]
	if !ok {
		log.Error().Msgf("Error  challenge could not be found from all available challenges ")
	}
	return ch
}

func (t *Team) GetDisabledChalMap() map[string][]string {
	t.m.Lock()
	defer t.m.Unlock()
	return t.disabledChallenges
}

func (t *Team) UpdateAllChallenges(challenges map[string][]string) {
	t.m.Lock()
	defer t.m.Unlock()
	for parent, child := range challenges {
		t.allChallenges[parent] = child
	}
}

func (t *Team) ManageDisabledChals(parentTag string) bool {
	t.m.Lock()
	defer t.m.Unlock()
	// this part is used for challenges to be run
	_, ok := t.disabledChallenges[parentTag]
	if ok {
		log.Debug().Msgf("Challenge   [ %s ]  is removed from disabled challenges .... ", parentTag)
		delete(t.disabledChallenges, parentTag)
		return true // returning true challenge is removed from disabledchal
	}

	// this part is used for challenges to stop
	ch, ok := t.allChallenges[parentTag]
	if ok {
		t.disabledChallenges[parentTag] = ch
		log.Debug().Msgf("Challenge [ %s ]  is added to disabled challenges .... ", parentTag)
		return false
	}
	return false

}

func (t *Team) AddDisabledChal(parentTag string) {
	t.m.Lock()
	defer t.m.Unlock()
	ch, ok := t.allChallenges[parentTag]
	if ok {
		t.disabledChallenges[parentTag] = ch
	}
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

func (t *Team) SetHostsInfo(hostInfo []string) {
	t.m.Lock()
	defer t.m.Unlock()
	t.hostsInfo = hostInfo
}

func (t *Team) GetHostsInfo() []string {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.hostsInfo
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

func (t *Team) AddChallenge(c Challenge) (string, error) {
	t.m.Lock()
	for _, chal := range t.challenges {
		if chal.Tag == c.Tag {
			t.m.Unlock()
			return "", ErrChallengeDuplicate
		}
	}

	f := c.Value

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

func (t *Team) VerifyFlag(tag Challenge, f string) error {
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
		return fmt.Errorf("Flag for challenge [ %s ] is already completed!", chal.Tag)
	}
	now := time.Now()
	chal.CompletedAt = &now
	t.challenges[f] = chal

	err := t.UpdateTeamSolvedChallenges(chal)
	if err != nil {
		t.m.Unlock()
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

func (t *Team) UpdatePass(pass, passRepeat, evTag string) error {
	t.m.RLock()
	ctx := context.Background()
	if pass != passRepeat {
		t.m.RUnlock()
		return fmt.Errorf("Passwords DOES NOT match !")
	}

	hPass, _ := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	resp, err := t.dbc.GetEventID(ctx, &pbc.GetEventIDReq{EventTag: evTag})
	if err != nil {
		t.m.RUnlock()
		return err
	}
	eventID := resp.EventID

	t.hashedPassword = string(hPass)

	_, err = t.dbc.UpdateTeamPassword(ctx, &pbc.UpdateTeamPassRequest{
		EncryptedPass: string(hPass),
		TeamID:        t.ID(),
		EventID:       int32(eventID),
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

func CopyMap(m map[string][]string) map[string][]string {
	nm := make(map[string][]string)
	for k, v := range m {
		_, ok := nm[k]
		if !ok {
			nm[k] = v
		}
	}
	return nm
}
