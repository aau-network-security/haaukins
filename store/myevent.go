package store

import (
	"context"
	"fmt"
	"github.com/aau-network-security/haaukins"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/rs/zerolog/log"
	"time"
)




type Event struct {
	dbc pbc.StoreClient
	TeamStore
	EventConfig
}

func (e Event) Read() EventConfig {
	panic("implement me")
}

func (e Event) SetCapacity(n int) error {
	panic("implement me")
}

func (e Event) Finish(time.Time) error {
	panic("implement me")
}

func (e Event) ArchiveDir() string {
	panic("implement me")
}

func (e Event) Archive() error {
	panic("implement me")
}

func NewEventStore (conf EventConfig, dbc pbc.StoreClient) (Event, error){
	ctx := context.Background()
	ts := NewTeamStore()

	teamsDB, err := dbc.GetEventTeams(ctx, &pbc.GetEventTeamsRequest{EventTag: string(conf.Tag)})
	if err != nil{
		return Event{}, err
	}
	fmt.Println("creating event store for the event: " + string(conf.Tag))
	for _, teamDB := range teamsDB.Teams{
		fmt.Println(string(conf.Tag) + "------" +teamDB.Id)
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

