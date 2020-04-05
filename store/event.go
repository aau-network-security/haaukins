// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

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
	Name           string     `yaml:"name"`
	Tag            Tag        `yaml:"tag"`
	Available      int        `yaml:"available"`
	Capacity       int        `yaml:"capacity"`
	Lab            Lab        `yaml:"lab"`
	StartedAt      *time.Time `yaml:"started-at,omitempty"`
	FinishExpected *time.Time `yaml:"finish-req,omitempty"`
	FinishedAt     *time.Time `yaml:"finished-at,omitempty"`
	CreatedBy      string     `yaml:"created-by,omitempty"`
	IsBooked       bool       `yaml: "is-booked", omitempty`
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
	Id               string            `yaml:"id"`
	Email            string            `yaml:"email"`
	Name             string            `yaml:"name"`
	HashedPassword   string            `yaml:"hashed-password"`
	SolvedChallenges []Challenge       `yaml:"solved-challenges,omitempty"`
	Metadata         map[string]string `yaml:"metadata,omitempty"`
	CreatedAt        *time.Time        `yaml:"created-at,omitempty"`
	ChalMap          map[Tag]Challenge `yaml:"-"`
	AccessedAt       *time.Time        `yaml:"accessed-at,omitempty"`
}

func NewTeam(email, name, password string, chals ...Challenge) Team {
	now := time.Now()

	hashedPassword := fmt.Sprintf("%x", sha256.Sum256([]byte(password)))
	email = strings.ToLower(email)

	t := Team{
		Id:             uuid.New().String()[0:8],
		Email:          email,
		Name:           name,
		HashedPassword: hashedPassword,
		CreatedAt:      &now,
		AccessedAt:     nil,
	}
	for _, chal := range chals {
		t.AddChallenge(chal)
		log.Info().Str("chal-tag", string(chal.FlagTag)).
			Str("chal-val", chal.FlagValue).
			Msgf("Flag is created for team %s ", t.Name)
	}
	return t
}

func (t *Team) IsCorrectFlag(tag Tag, v string) error {
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

	t.SolvedChallenges = append(t.SolvedChallenges, c)
	t.AddChallenge(c)

	return nil
}

func (t *Team) AddMetadata(key, value string) {
	if t.Metadata == nil {
		t.Metadata = map[string]string{}
	}
	t.Metadata[key] = value
}

func (t *Team) DataCollection() bool {
	if t.Metadata == nil {
		return false
	}

	v, ok := t.Metadata["consent"]
	if !ok {
		return false
	}

	return v == "ok"
}

func (t *Team) AddChallenge(c Challenge) {
	if t.ChalMap == nil {
		t.ChalMap = map[Tag]Challenge{}
	}
	t.ChalMap[c.FlagTag] = c
}

func (t *Team) DataConsent() bool {
	if t.Metadata == nil {
		return false
	}
	v, ok := t.Metadata["consent"]
	if !ok {
		return false
	}
	return v == "ok"
}

func (t *Team) SetAccessed(ti time.Time) {
	t.AccessedAt = &ti
}

type TeamStore interface {
	CreateTeam(Team) error
	GetTeamByToken(string) (Team, error)
	GetTeamByEmail(string) (Team, error)
	GetTeamByName(string) (Team, error)
	GetTeams() []Team
	SaveTeam(Team) error
	UpdateTeamAccessed(string, time.Time) (Team, error)
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
			if err := ts.CreateTeam(t); err != nil {
				log.Error().Msgf("Error on creating team %s", err)
			}
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

func (es *teamstore) UpdateTeamAccessed(id string, ti time.Time) (Team, error) {
	es.m.Lock()
	defer es.m.Unlock()

	t, ok := es.teams[id]
	if !ok {
		return Team{}, UnknownTeamErr
	}

	t.SetAccessed(ti)
	es.teams[id] = t

	return t, es.RunHooks()
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
	SetBooking(bool) EventConfig
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

func (es *eventconfigstore) SetBooking(isBooked bool) EventConfig {
	log.Debug().Str("Event ", es.conf.Name).
		Str("Tag ", string(es.conf.Tag)).
		Str("Started at ", es.conf.StartedAt.String()).
		Str("Created by/[Requested by]", es.conf.CreatedBy).
		Msg("isBooked updated ")

	es.m.Lock()
	defer es.m.Unlock()
	es.conf.IsBooked = isBooked

	return es.conf
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
	UpdateEventFile(EventConfig) (EventFile, error)
	GetUnfinishedEvents() ([]EventFile, error)
}

type eventfilehub struct {
	m    sync.Mutex
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

type Archiver interface {
	ArchiveDir() string
	Archive() error
}

type EventFile interface {
	TeamStore
	EventConfigStore
	Archiver
}

type eventfile struct {
	m        sync.Mutex
	file     RawEventFile
	dir      string
	filename string

	TeamStore
	EventConfigStore
}

func NewEventFile(dir string, filename string, file RawEventFile) *eventfile {
	ef := &eventfile{
		dir:      dir,
		filename: filename,
		file:     file,
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

	return ioutil.WriteFile(ef.path(), bytes, 0644)
}

func (ef *eventfile) delete() error {
	return os.Remove(ef.path())
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

func (ef *eventfile) path() string {
	return filepath.Join(ef.dir, ef.filename)
}

func (ef *eventfile) ArchiveDir() string {
	parts := strings.Split(ef.filename, ".")
	relativeDir := strings.Join(parts[:len(parts)-1], ".")
	return filepath.Join(ef.dir, relativeDir)
}

func (ef *eventfile) Archive() error {
	ef.m.Lock()
	defer ef.m.Unlock()

	if _, err := os.Stat(ef.ArchiveDir()); os.IsNotExist(err) {
		if err := os.MkdirAll(ef.ArchiveDir(), os.ModePerm); err != nil {
			return err
		}
	}

	cpy := eventfile{
		file:     ef.file,
		dir:      ef.ArchiveDir(),
		filename: "config.yml",
	}

	cpy.file.Teams = []Team{}
	for _, t := range ef.GetTeams() {
		t.Name = ""
		t.Email = ""
		t.HashedPassword = ""
		cpy.file.Teams = append(cpy.file.Teams, t)
	}
	cpy.save()

	if err := ef.delete(); err != nil {
		log.Warn().Msgf("Failed to delete old event file: %s", err)
	}

	return nil
}

func getFileNameForEvent(path string, tag Tag) (string, error) {
	now := time.Now().Format("02-01-06")
	dirname := fmt.Sprintf("%s-%s", tag, now)
	filename := fmt.Sprintf("%s.yml", dirname)

	_, dirErr := os.Stat(filepath.Join(path, dirname))
	_, fileErr := os.Stat(filepath.Join(path, filename))

	if os.IsNotExist(fileErr) && os.IsNotExist(dirErr) {
		return filename, nil
	}

	for i := 1; i < 999; i++ {
		dirname := fmt.Sprintf("%s-%s-%d", tag, now, i)
		filename := fmt.Sprintf("%s.yml", dirname)

		_, dirErr := os.Stat(filepath.Join(path, dirname))
		_, fileErr := os.Stat(filepath.Join(path, filename))

		if os.IsNotExist(fileErr) && os.IsNotExist(dirErr) {
			return filename, nil
		}
	}

	return "", fmt.Errorf("unable to get filename for event")
}

func (esh *eventfilehub) CreateEventFile(conf EventConfig) (EventFile, error) {
	filename, err := getFileNameForEvent(esh.path, conf.Tag)
	if err != nil {
		return nil, err
	}

	ef := NewEventFile(esh.path, filename, RawEventFile{EventConfig: conf})
	if err := ef.save(); err != nil {
		return nil, err
	}

	return ef, nil
}

func (esh *eventfilehub) UpdateEventFile(conf EventConfig) (EventFile, error) {
	esh.m.Lock()
	defer esh.m.Unlock()
	// filename is created for booked events...
	startedTime := conf.StartedAt.Format("02-01-06")
	dirname := fmt.Sprintf("%s-%s", conf.Tag, startedTime)
	filename := fmt.Sprintf("%s.yml", dirname)
	filename = filepath.Join(esh.path, filename)

	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var updatedEventFile RawEventFile
	err = yaml.Unmarshal(f, &updatedEventFile)
	if err != nil {
		return nil, err
	}
	updatedEventFile.IsBooked = false
	conf.IsBooked = false
	d, err := yaml.Marshal(&updatedEventFile)
	if err != nil {
		log.Error().Msgf("error: %v", err)
	}

	err = ioutil.WriteFile(filename, d, 0644)
	if err != nil {
		log.Error().Msgf("error: %v", err)
	}
	log.Debug().Msgf("Event file %s is updated, auto start event is triggered ", filename)
	return NewEventFile(esh.path, filename, RawEventFile{EventConfig: conf}), nil

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
			if ef.IsBooked {
				events = append(events, &eventfile{file: ef, EventConfigStore: &eventconfigstore{
					m:     sync.Mutex{},
					conf:  ef.EventConfig,
					hooks: nil,
				}})

			}
			// could be needed to check number of events for users who do not hold privilege
			// do not start events which are expired when getting unfinished events
			if ef.FinishExpected != nil && ef.FinishExpected.After(time.Now()) && (!ef.IsBooked || ef.StartedAt.Before(time.Now())) {
				dir, filename := filepath.Split(path)
				log.Debug().Str("Name", ef.Name).
					Str("Event Tag", string(ef.Tag)).
					Str("Started date", ef.StartedAt.String()).
					Str("Expected finish date", ef.FinishExpected.String()).
					Msgf("Found unfinished event")
				events = append(events, NewEventFile(dir, filename, ef))
			}

		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return events, nil
}
