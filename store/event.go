package store

import (
	"errors"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	TeamExistsErr   = errors.New("Team already exists")
	UnknownTeamErr  = errors.New("Unknown team")
	UnknownTokenErr = errors.New("Unknown token")
	EmptyTokenErr   = errors.New("Token cannot be empty")
)

type Task struct {
	ExerciseTag string     `yaml:"extag"`
	CompletedAt *time.Time `yaml:"completed-at,omitempty"`
}

type Team struct {
	Email          string `yaml:"email"`
	Name           string `yaml:"name"`
	HashedPassword string `yaml:"hashed-password"`
	Tasks          []Task `yaml:"tasks"`
}

func NewTeam(email, name, password string, tasks []Task) (Team, error) {
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

func (t Team) SolveTaskByTag(tag string) error {
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

type EventStore interface {
	CreateTeam(Team) error
	GetTeamByToken(string) (Team, error)
	SaveTeam(Team) error

	CreateTokenForTeam(string, Team) error
	DeleteToken(string) error
}

type eventstore struct {
	m sync.RWMutex

	hooks  []func([]Team) error
	teams  map[string]Team
	tokens map[string]string
}

func NewEventStore(hooks ...func([]Team) error) EventStore {
	return &eventstore{
		hooks:  hooks,
		teams:  map[string]Team{},
		tokens: map[string]string{},
	}
}

func (es *eventstore) CreateTeam(t Team) error {
	es.m.Lock()
	defer es.m.Unlock()

	if _, ok := es.teams[t.Email]; ok {
		return TeamExistsErr
	}

	es.teams[t.Email] = t

	return es.RunHooks()
}

func (es *eventstore) SaveTeam(t Team) error {
	es.m.Lock()
	defer es.m.Unlock()

	if _, ok := es.teams[t.Email]; !ok {
		return UnknownTeamErr
	}

	es.teams[t.Email] = t

	return es.RunHooks()
}

func (es *eventstore) CreateTokenForTeam(token string, in Team) error {
	es.m.Lock()
	defer es.m.Unlock()

	if token == "" {
		return EmptyTokenErr
	}

	t, ok := es.teams[in.Email]
	if !ok {
		return UnknownTeamErr
	}

	es.tokens[token] = t.Email

	return nil
}

func (es *eventstore) DeleteToken(token string) error {
	es.m.Lock()
	defer es.m.Unlock()

	delete(es.tokens, token)

	return nil
}

func (es *eventstore) GetTeamByToken(token string) (Team, error) {
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

func (es *eventstore) RunHooks() error {
	var teams []Team
	for _, t := range es.teams {
		teams = append(teams, t)
	}

	for _, h := range es.hooks {
		if err := h(teams); err != nil {
			return err
		}
	}

	return nil
}
