package store

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	yaml "gopkg.in/yaml.v2"
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
	Tag        string     `yaml:"tag"`
	Buffer     int        `yaml:"buffer"`
	Capacity   int        `yaml:"capacity"`
	Lab        Lab        `yaml:"lab"`
	StartedAt  *time.Time `yaml:"started-at,omitempty"`
	FinishedAt *time.Time `yaml:"finished-at,omitempty"`
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
	ExerciseTag Tag        `yaml:"tag"`
	CompletedAt *time.Time `yaml:"completed-at,omitempty"`
}

type Team struct {
	Email          string `yaml:"email"`
	Name           string `yaml:"name"`
	HashedPassword string `yaml:"hashed-password"`
	Tasks          []Task `yaml:"tasks"`
}

func NewTeam(email, name, password string, tasks ...Task) (Team, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Team{}, err
	}

	return Team{
		Email:          email,
		Name:           name,
		HashedPassword: string(hashedBytes[:]),
		Tasks:          tasks,
	}, nil
}

func (t Team) SolveTaskByTag(tag Tag) error {
	var task *Task
	for i, ta := range t.Tasks {
		if ta.ExerciseTag == tag {
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
}

func NewTeamStore(hooks ...func([]Team) error) TeamStore {
	return &teamstore{
		hooks:  hooks,
		teams:  map[string]Team{},
		tokens: map[string]string{},
	}
}

func (es *teamstore) CreateTeam(t Team) error {
	es.m.Lock()
	defer es.m.Unlock()

	if _, ok := es.teams[t.Email]; ok {
		return TeamExistsErr
	}

	es.teams[t.Email] = t

	return es.RunHooks()
}

func (es *teamstore) SaveTeam(t Team) error {
	es.m.Lock()
	defer es.m.Unlock()

	if _, ok := es.teams[t.Email]; !ok {
		return UnknownTeamErr
	}

	es.teams[t.Email] = t

	return es.RunHooks()
}

func (es *teamstore) CreateTokenForTeam(token string, in Team) error {
	es.m.Lock()
	defer es.m.Unlock()

	if token == "" {
		return &EmptyVarErr{"Token"}
	}

	t, ok := es.teams[in.Email]
	if !ok {
		return UnknownTeamErr
	}

	es.tokens[token] = t.Email

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

func (es *teamstore) GetTeamByToken(token string) (Team, error) {
	es.m.RLock()
	defer es.m.RUnlock()

	m, ok := es.tokens[token]
	if !ok {
		return Team{}, UnknownTokenErr
	}

	t, ok := es.teams[m]
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
	GetUnfinishedEvents() ([]Event, error)
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

type eventFile struct {
	Event
	Teams []Team `yaml:"teams,omitempty"`
}

func getFileNameForEvent(path string, tag string) (string, error) {
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

	ef := eventFile{conf, []Team{}}
	var m sync.Mutex

	save := func() error {
		bytes, err := yaml.Marshal(ef)
		if err != nil {
			return err
		}

		return ioutil.WriteFile(filename, bytes, 0644)
	}

	teamHook := func(teams []Team) error {
		m.Lock()
		defer m.Unlock()

		ef.Teams = teams

		return save()
	}

	eventHook := func(c Event) error {
		m.Lock()
		defer m.Unlock()

		ef = eventFile{c, ef.Teams}

		return save()
	}

	ts := NewTeamStore(teamHook)

	if err := eventHook(conf); err != nil {
		return nil, err
	}

	return NewEventStore(conf, ts, eventHook), nil
}

func (esh *eventstorehub) GetUnfinishedEvents() ([]Event, error) {
	var events []Event
	err := filepath.Walk(esh.path, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".yml" {
			f, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			var e eventFile
			err = yaml.Unmarshal(f, &e)
			if err != nil {
				return err
			}

			if e.FinishedAt == nil {
				events = append(events, e.Event)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return events, nil
}
