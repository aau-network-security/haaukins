package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/aau-network-security/haaukins"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/amigo"
)

func main() {
	chals := []haaukins.Challenge{{"HB", "Heartbleed"},{"AAA", "Test"}}
	team := haaukins.NewTeam("test1@test.dk", "TestingTeamOne", "123456")
	f, err := team.AddChallenge(chals[0])
	team.AddChallenge(chals[1])
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(f.String())

	team2 := haaukins.NewTeam("test@test.dk", "TestingTeam2", "123456")
	team2.AddChallenge(chals[0])
	team2.AddChallenge(chals[1])
	ts := store.NewTeamStore(team, team2)
	am := amigo.NewAmigo(ts, chals, "abcde")

	log.Fatal(http.ListenAndServe(":8080", am.Handler()))
}
