// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package store

import (
	"errors"
	"time"
)

const (
	ID_KEY       = "I"
	TEAMNAME_KEY = "TN"
	token_key	 = "testing"
	displayTimeFormat = "2006-01-02 15:04:05"
)

var (
	TeamExistsErr       = errors.New("Team already exists")
	UnknownTeamErr      = errors.New("Unknown team")
	UnknownTokenErr     = errors.New("Unknown token")
	NoFrontendErr       = errors.New("lab requires at least one frontend")
	InvalidFlagValueErr = errors.New("Incorrect value for flag")
	UnknownChallengeErr = errors.New("Unknown challenge")
)

type RawEvent struct {
	Name			string
	Tag				string
	Available		int32
	Capacity		int32
	Exercises  		string
	Frontends 		string
	StartedAt		string
	FinishExpected 	string
}

type EventConfig struct {
	Name       string     `yaml:"name"`
	Tag        Tag        `yaml:"tag"`
	Available  int        `yaml:"available"`
	Capacity   int        `yaml:"capacity"`
	Lab        Lab        `yaml:"lab"`
	StartedAt  *time.Time `yaml:"started-at,omitempty"`
	FinishExpected  *time.Time `yaml:"finish-req,omitempty"`
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
	Id               string
	Email            string
	Name             string
	HashedPassword   string
	SolvedChallenges []Challenge
	Metadata         map[string]string
	CreatedAt        *time.Time
	ChalMap          map[Tag]Challenge
	AccessedAt       *time.Time
}



//func (ef *eventfile) save() error {
//	bytes, err := yaml.Marshal(ef.file)
//	if err != nil {
//		return err
//	}
//
//	return ioutil.WriteFile(ef.path(), bytes, 0644)
//}
//
//func (ef *eventfile) delete() error {
//	return os.Remove(ef.path())
//}
//
//
//func (ef *eventfile) saveEventConfig(conf EventConfig) error {
//	ef.m.Lock()
//	defer ef.m.Unlock()
//
//	ef.file.EventConfig = conf
//
//	return ef.save()
//}
//
//func (ef *eventfile) path() string {
//	return filepath.Join(ef.dir, ef.filename)
//}
//
//func (ef *eventfile) ArchiveDir() string {
//	parts := strings.Split(ef.filename, ".")
//	relativeDir := strings.Join(parts[:len(parts)-1], ".")
//	return filepath.Join(ef.dir, relativeDir)
//}
//
//func (ef *eventfile) Archive() error {
//	ef.m.Lock()
//	defer ef.m.Unlock()
//
//	if _, err := os.Stat(ef.ArchiveDir()); os.IsNotExist(err) {
//		if err := os.MkdirAll(ef.ArchiveDir(), os.ModePerm); err != nil {
//			return err
//		}
//	}
//
//	//cpy := eventfile{
//	//	file:     ef.file,
//	//	dir:      ef.ArchiveDir(),
//	//	filename: "config.yml",
//	//}
//	//
//	//cpy.file.Teams = []*haaukins.Team{}
//	//for _, t := range ef.GetTeams() {
//	//
//	//	cpy.file.Teams = append(cpy.file.Teams, t)
//	//}
//	//cpy.save()
//
//	if err := ef.delete(); err != nil {
//		log.Warn().Msgf("Failed to delete old event file: %s", err)
//	}
//
//	return nil
//}
//
