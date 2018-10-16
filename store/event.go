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
	"gopkg.in/yaml.v2"
	"github.com/rs/zerolog/log"
	)

type EmptyVarErr struct {
	Variable string
}

func (eve *EmptyVarErr) Error() string { return fmt.Sprintf("%s cannot be empty", eve.Variable) }

var (
	TeamExistsErr   = errors.New("Team already exists")
	UnknownTeamErr  = errors.New("Unknown team")
	UnknownTokenErr = errors.New("Unknown token")
	NoFrontendErr   = errors.New("lab requires at least one frontend")
)

type Event struct {
	Name       string     `yaml:"name"`
	Tag        Tag        `yaml:"tag"`
	Buffer     int        `yaml:"buffer"`
	Capacity   int        `yaml:"capacity"`
	Lab        Lab        `yaml:"lab"`
	StartedAt  *time.Time `yaml:"started-at,omitempty"`
	FinishedAt *time.Time `yaml:"finished-at,omitempty"`
	Teams      []Team     `yaml:"teams,omitempty"`
}

func (e Event) Validate() error {
	if e.Name == "" {
		return &EmptyVarErr{"Name"}
	}

	if e.Tag == "" {
		return &EmptyVarErr{"Tag"}
	}

	if len(e.Lab.Exercises) == 0 {
		return &EmptyVarErr{"Exercises"}
	}

	if len(e.Lab.Frontends) == 0 {
		return &EmptyVarErr{"Frontends"}
	}

	return nil
}

type Lab struct {
	Frontends []string `yaml:"frontends"`
	Exercises []Tag    `yaml:"exercises"`
}

type Task struct {
	OwnerID     string     `yaml:"-"`
	FlagTag     Tag        `yaml:"tag,omitempty"`
	CompletedAt *time.Time `yaml:"completed-at,omitempty"`
}

type Team struct {
	Id             string `yaml:"id"`
	Email          string `yaml:"email"`
	Name           string `yaml:"name"`
	HashedPassword string `yaml:"hashed-password"`
	Tasks          []Task `yaml:"tasks,omitempty"`
}

func NewTeam(email, name, password string, tasks ...Task) Team {
	hashedPassword := fmt.Sprintf("%x", sha256.Sum256([]byte(password)))

	email = strings.ToLower(email)
	return Team{
		Id:             uuid.New().String()[0:8],
		Email:          email,
		Name:           name,
		HashedPassword: hashedPassword,
		Tasks:          tasks,
	}
}

func (t Team) SolveTaskByTag(tag Tag) error {
	var task *Task
	for i, ta := range t.Tasks {
		if ta.FlagTag == tag {
			task = &t.Tasks[i]
		}
	}

	if task == nil {
		return &UnknownExerTagErr{tag}
	}

	now := time.Now()
	task.CompletedAt = &now

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

func NewTeamStore(teams []Team, hooks ...func([]Team) error) TeamStore {
	ts := &teamstore{
		hooks:  hooks,
		teams:  map[string]Team{},
		tokens: map[string]string{},
		names:  map[string]string{},
		emails: map[string]string{},
	}
	for _, t := range teams {
		ts.CreateTeam(t)
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
		return &EmptyVarErr{"Token"}
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

type EventStore interface {
	Read() Event
	SetCapacity(n int) error
	Finish(time.Time) error
	TeamStore
}

type eventstore struct {
	m sync.Mutex

	hooks []func(Event) error
	conf  Event
	TeamStore
}

func NewEventStore(conf Event, ts TeamStore, hooks ...func(Event) error) EventStore {
	return &eventstore{
		hooks:     hooks,
		conf:      conf,
		TeamStore: ts,
	}
}

func (es *eventstore) Read() Event {
	es.m.Lock()
	defer es.m.Unlock()

	return es.conf
}

func (es *eventstore) SetCapacity(n int) error {
	es.m.Lock()
	defer es.m.Unlock()

	es.conf.Capacity = n

	return es.RunHooks()
}

func (es *eventstore) Finish(t time.Time) error {
	es.m.Lock()
	defer es.m.Unlock()

	es.conf.FinishedAt = &t

	return es.RunHooks()
}

func (es *eventstore) RunHooks() error {
	for _, h := range es.hooks {
		if err := h(es.conf); err != nil {
			return err
		}
	}

	return nil
}

type EventStoreHub interface {
	CreateEventStore(Event) (EventStore, error)
	GetUnfinishedEvents() ([]EventStore, error)
}

type eventstorehub struct {
	m sync.Mutex

	path string
}

func NewEventStoreHub(path string) (EventStoreHub, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return nil, err
		}
	}

	return &eventstorehub{
		path: path,
	}, nil
}

func (ef *eventFile) save() error {
	bytes, err := yaml.Marshal(ef)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(ef.path, bytes, 0644)
}

func (ef *eventFile) SaveTeams(teams []Team) error {
	ef.m.Lock()
	defer ef.m.Unlock()

	ef.Teams = teams

	return ef.save()
}

func (ef *eventFile) SaveEvent(event Event) error {
	ef.m.Lock()
	defer ef.m.Unlock()

	ef.Event = event

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

func (esh *eventstorehub) CreateEventStore(conf Event) (EventStore, error) {
	filename, err := getFileNameForEvent(esh.path, conf.Tag)
	if err != nil {
		return nil, err
	}

	ef := &eventFile{
		Event:  conf,
		Teams: []Team{},
		path:  filename,
	}

	ts := NewTeamStore(nil, ef.SaveTeams)

	if err := ef.SaveEvent(conf); err != nil {
		return nil, err
	}

	return NewEventStore(conf, ts, ef.SaveEvent), nil
}

func (esh *eventstorehub) GetUnfinishedEvents() ([]EventStore, error) {
	var events []EventStore
	err := filepath.Walk(esh.path, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".yml" {
			log.Debug().Msgf("Found unfinished event configuration: %s", path)
			f, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			var ef eventFile
			err = yaml.Unmarshal(f, &ef)
			if err != nil {
				return err
			}
			ef.path = path

			if ef.FinishedAt == nil {
				ts := NewTeamStore(ef.Teams, ef.SaveTeams)
				e := NewEventStore(ef.Event, ts, ef.SaveEvent)
				events = append(events, e)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return events, nil
}
