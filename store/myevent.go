package store

import (
	"context"
	"fmt"
	"github.com/aau-network-security/haaukins"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/dgrijalva/jwt-go"
	"github.com/rs/zerolog/log"
	"time"
)

type Event struct {
	dbc pbc.StoreClient
	TeamStore
	EventConfig
}

// Change the Capacity of the event and update the DB
func (e Event) SetCapacity(n int) error {
	// todo might be usefull to have it, create a rpc message for it in case
	panic("implement me")
}

func (e Event) Finish(time time.Time) error {

	_, err := e.dbc.UpdateEventFinishDate(context.Background(), &pbc.UpdateEventRequest{
		EventId:              string(e.Tag),
		FinishedAt:           time.Format(displayTimeFormat),
	})
	if err != nil {

		return err
	}
	return nil
}

// Create the EventSore for the event. It contains:
// The connection with the DB
// A new TeamStore that contains all the teams retrieved from the DB (if no teams are retrieved the TeamStore will be empty)
// The EventConfiguration
func NewEventStore (conf EventConfig, dbc pbc.StoreClient) (Event, error){
	ctx := context.Background()
	ts := NewTeamStore(conf, dbc)

	teamsDB, err := dbc.GetEventTeams(ctx, &pbc.GetEventTeamsRequest{EventTag: string(conf.Tag)})
	if err != nil{
		return Event{}, err
	}
	fmt.Println("creating event store for the event: " + string(conf.Tag))
	for _, teamDB := range teamsDB.Teams{
		fmt.Println(teamDB)
		team := haaukins.NewTeam(teamDB.Email, teamDB.Name, "", teamDB.Id, teamDB.HashPassword)
		teamToken, err := GetTokenForTeam([]byte(token_key), team)
		if err != nil {
			log.Debug().Msgf("Error in getting token for team %s", team.Name())
		}
		ts.tokens[teamToken]=team.ID()
		ts.emails[team.Email()]=team.ID()
		ts.teams[team.ID()]=team
	}

	return Event{
		dbc:         dbc,
		TeamStore:   ts,
		EventConfig: conf,
	}, nil
}

func GetTokenForTeam(key []byte, t *haaukins.Team) (string, error) {
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