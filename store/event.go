// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	wg "github.com/aau-network-security/haaukins/network/vpn"
	jwt "github.com/golang-jwt/jwt/v4"

	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/rs/zerolog/log"
)

const (
	ID_KEY            = "I"
	TEAMNAME_KEY      = "TN"
	token_key         = "testing"
	displayTimeFormat = time.RFC3339
)

var (
	UnknownTeamErr  = errors.New("Unknown team")
	UnknownTokenErr = errors.New("Unknown token")
)

type EventConfig struct {
	Name               string
	Host               string
	Tag                Tag
	Available          int
	Capacity           int
	Lab                Lab
	StartedAt          *time.Time
	FinishExpected     *time.Time
	FinishedAt         *time.Time
	Status             int32
	CreatedBy          string
	OnlyVPN            int32
	VPNAddress         string
	EndPointPort       int
	DisabledChallenges map[string][]string // list of disabled children challenge tags to be used for amigo frontend ...
	AllChallenges      map[string][]string
	SecretKey          string // secret key is a key which is defined by event creator to setup events which are accessible only with signup key

}

type Lab struct {
	Frontends         []InstanceConfig
	Exercises         []Tag
	DisabledExercises []Tag
}

func (e EventConfig) Validate() error {

	if e.Name == "" {
		return &EmptyVarErr{Var: "Name", Type: "Event"}
	}

	if e.Tag == "" {
		return &EmptyVarErr{Var: "Tag", Type: "Event"}
	}

	if e.CreatedBy == "" {
		return &EmptyVarErr{Var: "User", Type: "Event"}
	}

	if len(e.Lab.Exercises) == 0 {
		return &EmptyVarErr{Var: "Exercises", Type: "Event"}
	}

	if len(e.Lab.Frontends) == 0 {
		return &EmptyVarErr{Var: "Frontends", Type: "Event"}
	}

	return nil
}

type Event struct {
	Dir string
	dbc pbc.StoreClient
	TeamStore
	EventConfig
	wg.WireGuardConfig
}

// Change the Capacity of the event and update the DB
func (e Event) SetCapacity(n int) error {
	// todo might be usefull to have it, create a rpc message for it in case
	panic("implement me")
}

func (e Event) Finish(newTag string, time time.Time) error {

	_, err := e.dbc.UpdateCloseEvent(context.Background(), &pbc.UpdateEventRequest{
		OldTag:     string(e.Tag),
		NewTag:     newTag,
		FinishedAt: time.Format(displayTimeFormat),
	})
	if err != nil {

		return fmt.Errorf("error on closing event on the store %v", err)
	}
	return nil
}

// SetStatus will set status of event on db
func (e Event) SetStatus(eventTag string, status int32) error {
	_, err := e.dbc.SetEventStatus(context.Background(), &pbc.SetEventStatusRequest{
		EventTag: eventTag,
		Status:   status,
	})

	e.Status = status

	if err != nil {
		return err
	}
	return nil
}

// Create the EventSore for the event. It contains:
// The connection with the DB
// A new TeamStore that contains all the teams retrieved from the DB (if no teams are retrieved the TeamStore will be empty)
// The EventConfiguration
func NewEventStore(conf EventConfig, eDir string, dbc pbc.StoreClient) (Event, error) {
	ctx := context.Background()
	ts := NewTeamStore(conf, dbc)
	teamsDB, err := dbc.GetEventTeams(ctx, &pbc.GetEventTeamsRequest{EventTag: string(conf.Tag)})
	if err != nil {
		return Event{}, err
	}
	for _, teamDB := range teamsDB.Teams {
		lastAccessedTime, err := time.Parse(time.RFC3339, teamDB.LastAccess)
		if err != nil {
			log.Error().Msgf("[NewEventStore] Time parsing error %v", err)
		}
		// todo: add solved challenges to disabled challenges
		team := NewTeam(teamDB.Email, teamDB.Name, "",
			teamDB.Id, teamDB.HashPassword, teamDB.SolvedChallenges,
			lastAccessedTime.UTC(), conf.DisabledChallenges, conf.AllChallenges, dbc)
		teamToken, err := GetTokenForTeam([]byte(token_key), team)
		if err != nil {
			log.Debug().Msgf("Error in getting token for team %s", team.Name())
		}
		ts.tokens[teamToken] = team.ID()
		ts.names[team.Name()] = team.ID()
		ts.teams[team.ID()] = team
	}

	if _, err := os.Stat(eDir); os.IsNotExist(err) {
		if err := os.MkdirAll(eDir, os.ModePerm); err != nil {
			return Event{}, err
		}
	}

	return Event{
		Dir:         eDir,
		dbc:         dbc,
		TeamStore:   ts,
		EventConfig: conf,
	}, nil
}

func GetTokenForTeam(key []byte, t *Team) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		ID_KEY:       t.ID(),
		TEAMNAME_KEY: t.Name(),
	})
	tokenStr, err := token.SignedString(key)
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}

func GetDirNameForEvent(path string, tag Tag, date *time.Time) (string, error) {
	dirDate := date.Format("02-01-06")
	dirName := fmt.Sprintf("%s-%s", tag, dirDate)

	_, dirErr := os.Stat(filepath.Join(path, dirName))

	if os.IsNotExist(dirErr) {
		return dirName, nil
	}

	for i := 1; i < 999; i++ {
		dirname := fmt.Sprintf("%s-%s-%d", tag, dirDate, i)

		_, dirErr := os.Stat(filepath.Join(path, dirname))

		if os.IsNotExist(dirErr) {
			return dirName, nil
		}
	}

	if _, err := os.Stat(dirName); os.IsNotExist(err) {
		if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
			return "", err
		}
	}

	return dirName, nil
}
