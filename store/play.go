package store

import (
	"errors"
	yaml "gopkg.in/yaml.v2"
	"os"
	"sync"
)

var (
	PlayNoStepsErr = errors.New("play must contain at least one step")
)

var (
	PlayNotFound = errors.New("play could not be found")
)

type PlayStep struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Exercises   []Tag  `yaml:"exercises"`
	Requires    uint   `yaml:"requires"`
}

type Play struct {
	Tag   Tag        `yaml:"tag"`
	Steps []PlayStep `yaml:"steps"`
}

type PlayStore interface {
	GetPlayByTag(Tag) (Play, bool)
	GetPlays() []Play
}

func (p Play) Validate() error {
	if err := p.Tag.Validate(); err != nil {
		return err
	}

	if len(p.Steps) == 0 {
		return PlayNoStepsErr
	}

	return nil
}

// Creates a dummy play containing a single step with multiple exercises
func PlayFromExercises(name, description string, exercises ...Tag) Play {
	// TODO this is not a very nice solution
	t, _ := NewTag("anonymous")
	play := Play{
		Tag: t,
		Steps: []PlayStep{PlayStep{
			Name:        name,
			Description: description,
			Exercises:   exercises,
		}},
	}

	return play
}

type playstore struct {
	m      sync.Mutex
	tagmap map[Tag]*Play
	plays  []Play
	hooks  []func(PlayStore) error
}

func NewPlayStore(plays []Play, hooks ...func(PlayStore) error) PlayStore {
	ps := &playstore{
		tagmap: map[Tag]*Play{},
		plays:  plays,
		hooks:  hooks,
	}

	for i, p := range plays {
		ps.tagmap[p.Tag] = &plays[i]
	}

	return ps
}

// Parses path file and creates it if not exists
func NewPlayStoreFile(path string) (PlayStore, error) {
	var conf struct {
		Plays []Play `yaml:"plays"`
	}

	// Load file content
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		// File found
		err = yaml.NewDecoder(f).Decode(&conf)
		if err != nil {
			return nil, err
		}

		// Validate the loaded content
		for _, play := range conf.Plays {
			if err := play.Validate(); err != nil {
				return nil, err
			}
		}
	} else if !os.IsNotExist(err) {
		// Some error other than "Does not exits"
		return nil, err
	}

	var filelock sync.Mutex
	save := func(ps PlayStore) error {
		filelock.Lock()
		defer filelock.Unlock()

		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()

		conf.Plays = ps.GetPlays()

		return yaml.NewEncoder(f).Encode(&conf)
	}

	return NewPlayStore(conf.Plays, save), nil
}

func (ps *playstore) GetPlayByTag(tag Tag) (Play, bool) {
	ps.m.Lock()
	defer ps.m.Unlock()

	p, ok := ps.tagmap[tag]
	if !ok {
		return Play{}, false
	}

	return *p, true
}

// Returns all the plays in the store.
// Be aware that this is not a copy and must therefore not be edited.
// TODO If the returned array is to be edited return a copy instead like
// exercisestore
func (ps *playstore) GetPlays() []Play {
	return ps.plays
}
