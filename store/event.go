package store

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"crypto/sha256"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

var (
	TeamExistsErr       = errors.New("Team already exists")
	UnknownTeamErr      = errors.New("Unknown team")
	UnknownTokenErr     = errors.New("Unknown token")
	NoFrontendErr       = errors.New("lab requires at least one frontend")
	InvalidFlagValueErr = errors.New("Incorrect value for flag")
	UnknownChallengeErr = errors.New("Unknown challenge")
)

type EventConfig struct {
	Name       string     `yaml:"name"`
	Tag        Tag        `yaml:"tag"`
	Available  int        `yaml:"available"`
	Capacity   int        `yaml:"capacity"`
	Lab        Lab        `yaml:"lab"`
	StartedAt  *time.Time `yaml:"started-at,omitempty"`
	FinishedAt *time.Time `yaml:"finished-at,omitempty"`
}

type RawEventFile struct {
	EventConfig `yaml:",inline"`
	Teams       []Team `yaml:"teams,omitempty"`
}

func (e EventConfig) Validate() error {
	if e.Name == "" {
		return &EmptyVarErr{Var: "Name", Type: "Event"}
	}

	if e.Tag == "" {
		return &EmptyVarErr{Var: "Tag", Type: "Event"}
	}

	if len(e.Lab.Exercises) == 0 {
		return &EmptyVarErr{Var: "Exercises", Type: "Event"}
	}

	if len(e.Lab.Frontends) == 0 {
		return &EmptyVarErr{Var: "Frontends", Type: "Event"}
	}

	return nil
}

type Lab struct {
	Frontends []InstanceConfig `yaml:"frontends"`
	Exercises []Tag            `yaml:"exercises"`
}

type Challenge struct {
	OwnerID     string     `yaml:"-"`
	FlagTag     Tag        `yaml:"tag"`
	FlagValue   string     `yaml:"-"`
	CompletedAt *time.Time `yaml:"completed-at,omitempty"`
}

type Team struct {
	Id               string             `yaml:"id"`
	Email            string             `yaml:"email"`
	Name             string             `yaml:"name"`
	HashedPassword   string             `yaml:"hashed-password"`
	SolvedChallenges []Challenge        `yaml:"solved-challenges,omitempty"`
	CreatedAt        *time.Time         `yaml:"created-at,omitempty"`
	ChalMap          map[Tag]*Challenge `yaml:"-"`
	Logger           zerolog.Logger     `yaml:"-"`
}

func NewTeam(email, name, password string, chals ...Challenge) Team {
	now := time.Now()

	hashedPassword := fmt.Sprintf("%x", sha256.Sum256([]byte(password)))
	email = strings.ToLower(email)

	chalMap := map[Tag]*Challenge{}
	for _, chal := range chals {
		chalMap[chal.FlagTag] = &chal
	}

	logger := zerolog.New(os.Stdout)

	return Team{
		Id:             uuid.New().String()[0:8],
		Email:          email,
		Name:           name,
		HashedPassword: hashedPassword,
		ChalMap:        chalMap,
		CreatedAt:      &now,
		Logger:         logger,
	}
}

func (t Team) IsCorrectFlag(tag Tag, v string) error {
	c, ok := t.ChalMap[tag]
	if !ok {
		return UnknownChallengeErr
	}

	if c.FlagValue != v {
		return InvalidFlagValueErr
	}

	return nil
}

func (t *Team) SolveChallenge(tag Tag, v string) error {
	now := time.Now()

	if err := t.IsCorrectFlag(tag, v); err != nil {
		return err
	}

	c := t.ChalMap[tag]
	c.CompletedAt = &now

	t.SolvedChallenges = append(t.SolvedChallenges, *c)
	t.ChalMap[tag] = c

	return nil
}

type TeamStore interface {
	CreateTeam(Team) error
	GetTeamByToken(string) (Team, error)
	GetTeamByEmail(string) (Team, error)
	GetTeamByName(string) (Team, error)
	GetTeams() []Team
	SaveTeam(Team) error
	CreateTokenForTeam(string, Team) error
	DeleteToken(string) error
}

type teamstore struct {
	m sync.RWMutex

	hooks  []func([]Team) error
	teams  map[string]Team
	tokens map[string]string
	emails map[string]string
	names  map[string]string
}

type TeamStoreOpt func(ts *teamstore)

func WithTeams(teams []Team) func(ts *teamstore) {
	return func(ts *teamstore) {
		for _, t := range teams {
			ts.CreateTeam(t)
		}
	}
}

func WithPostTeamHook(hook func(teams []Team) error) func(ts *teamstore) {
	return func(ts *teamstore) {
		ts.hooks = append(ts.hooks, hook)
	}
}

func NewTeamStore(opts ...TeamStoreOpt) *teamstore {
	ts := &teamstore{
		hooks:  []func(teams []Team) error{},
		teams:  map[string]Team{},
		tokens: map[string]string{},
		names:  map[string]string{},
		emails: map[string]string{},
	}

	for _, opt := range opts {
		opt(ts)
	}

	return ts
}

func (es *teamstore) CreateTeam(t Team) error {
	es.m.Lock()
	defer es.m.Unlock()

	if _, ok := es.teams[t.Id]; ok {
		return TeamExistsErr
	}

	es.teams[t.Id] = t
	es.emails[t.Email] = t.Id
	es.names[t.Name] = t.Id

	return es.RunHooks()
}

func (es *teamstore) SaveTeam(t Team) error {
	es.m.Lock()
	defer es.m.Unlock()

	if _, ok := es.teams[t.Id]; !ok {
		return UnknownTeamErr
	}

	es.teams[t.Id] = t

	return es.RunHooks()
}

func (es *teamstore) CreateTokenForTeam(token string, in Team) error {
	es.m.Lock()
	defer es.m.Unlock()

	if token == "" {
		return &EmptyVarErr{Var: "Token"}
	}

	t, ok := es.teams[in.Id]
	if !ok {
		return UnknownTeamErr
	}

	es.tokens[token] = t.Id

	return nil
}

func (es *teamstore) DeleteToken(token string) error {
	es.m.Lock()
	defer es.m.Unlock()

	delete(es.tokens, token)

	return nil
}

func (es *teamstore) GetTeams() []Team {
	var teams []Team
	for _, t := range es.teams {
		teams = append(teams, t)
	}

	return teams
}

func (es *teamstore) GetTeamByEmail(email string) (Team, error) {
	es.m.RLock()
	defer es.m.RUnlock()

	id, ok := es.emails[email]
	if !ok {
		return Team{}, UnknownTokenErr
	}

	t, ok := es.teams[id]
	if !ok {
		return Team{}, UnknownTeamErr
	}

	return t, nil
}

func (es *teamstore) GetTeamByName(name string) (Team, error) {
	es.m.RLock()
	defer es.m.RUnlock()

	id, ok := es.names[name]
	if !ok {
		return Team{}, UnknownTokenErr
	}

	t, ok := es.teams[id]
	if !ok {
		return Team{}, UnknownTeamErr
	}

	return t, nil
}

func (es *teamstore) GetTeamByToken(token string) (Team, error) {
	es.m.RLock()
	defer es.m.RUnlock()

	id, ok := es.tokens[token]
	if !ok {
		return Team{}, UnknownTokenErr
	}

	t, ok := es.teams[id]
	if !ok {
		return Team{}, UnknownTeamErr
	}

	return t, nil
}

func (es *teamstore) RunHooks() error {
	teams := es.GetTeams()
	for _, h := range es.hooks {
		if err := h(teams); err != nil {
			return err
		}
	}

	return nil
}

type EventConfigStore interface {
	Read() EventConfig
	SetCapacity(n int) error
	Finish(time.Time) error
}

type eventconfigstore struct {
	m     sync.Mutex
	conf  EventConfig
	hooks []func(EventConfig) error
}

func NewEventConfigStore(conf EventConfig, hooks ...func(EventConfig) error) *eventconfigstore {
	return &eventconfigstore{
		conf:  conf,
		hooks: hooks,
	}
}

func (es *eventconfigstore) Read() EventConfig {
	es.m.Lock()
	defer es.m.Unlock()

	return es.conf
}

func (es *eventconfigstore) SetCapacity(n int) error {
	es.m.Lock()
	defer es.m.Unlock()

	es.conf.Capacity = n

	return es.runHooks()
}

func (es *eventconfigstore) Finish(t time.Time) error {
	es.m.Lock()
	defer es.m.Unlock()

	es.conf.FinishedAt = &t

	return es.runHooks()
}

func (es *eventconfigstore) runHooks() error {
	for _, h := range es.hooks {
		if err := h(es.conf); err != nil {
			return err
		}
	}

	return nil
}

type EventFileHub interface {
	CreateEventFile(EventConfig) (EventFile, error)
	GetUnfinishedEvents() ([]EventFile, error)
}

type eventfilehub struct {
	m sync.Mutex

	path string
}

func NewEventFileHub(path string) (EventFileHub, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return nil, err
		}
	}

	return &eventfilehub{
		path: path,
	}, nil
}

type EventFile interface {
	TeamStore
	EventConfigStore
}

type eventfile struct {
	m    sync.Mutex
	file RawEventFile
	path string

	TeamStore
	EventConfigStore
}

func NewEventFile(path string, file RawEventFile) *eventfile {
	ef := &eventfile{
		path: path,
		file: file,
	}

	ef.TeamStore = NewTeamStore(WithTeams(file.Teams), WithPostTeamHook(ef.saveTeams))
	ef.EventConfigStore = NewEventConfigStore(file.EventConfig, ef.saveEventConfig)

	return ef
}

func (ef *eventfile) save() error {
	bytes, err := yaml.Marshal(ef.file)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(ef.path, bytes, 0644)
}

func (ef *eventfile) saveTeams(teams []Team) error {
	ef.m.Lock()
	defer ef.m.Unlock()

	ef.file.Teams = teams

	return ef.save()
}

func (ef *eventfile) saveEventConfig(conf EventConfig) error {
	ef.m.Lock()
	defer ef.m.Unlock()

	ef.file.EventConfig = conf

	return ef.save()
}

func getFileNameForEvent(path string, tag Tag) (string, error) {
	now := time.Now().Format("02-01-06")
	filename := fmt.Sprintf("%s-%s.yml", tag, now)
	eventPath := filepath.Join(path, filename)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return eventPath, nil
	}

	for i := 1; i < 999; i++ {
		filename := fmt.Sprintf("%s-%s-%d.yml", tag, now, i)
		eventPath := filepath.Join(path, filename)

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			return eventPath, nil
		}
	}

	return "", fmt.Errorf("Unable to get filename for event")
}

func (esh *eventfilehub) CreateEventFile(conf EventConfig) (EventFile, error) {
	filename, err := getFileNameForEvent(esh.path, conf.Tag)
	if err != nil {
		return nil, err
	}

	ef := NewEventFile(filename, RawEventFile{EventConfig: conf})
	if err := ef.save(); err != nil {
		return nil, err
	}

	return ef, nil
}

func (esh *eventfilehub) GetUnfinishedEvents() ([]EventFile, error) {
	var events []EventFile
	err := filepath.Walk(esh.path, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".yml" {
			f, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			var ef RawEventFile
			err = yaml.Unmarshal(f, &ef)
			if err != nil {
				return err
			}

			if ef.FinishedAt == nil {
				log.Debug().Str("name", ef.Name).Msg("Found unfinished event")
				events = append(events, NewEventFile(path, ef))
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return events, nil
}
