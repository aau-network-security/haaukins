package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/store"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/aau-network-security/haaukins/svcs/amigo"
)

const (
	server             = "cli2.sec-aau.dk:50051"
	certFile           = ""
	certKey            = ""
	certCA             = ""
	tls                = true
	authKey            = ""
	signKey            = ""
	configExercisePath = "" //absolute path
	Running            = int32(0)
	Suspended          = int32(1)
	Booked             = int32(2)
)

func main() {

	dbConn := store.DBConfig{
		Grpc:     server,
		AuthKey:  authKey,
		SignKey:  signKey,
		Enabled:  tls,
		CertFile: certFile,
		CertKey:  certKey,
		CAFile:   certCA,
	}
	dbc, err := store.NewGRPClientDBConnection(dbConn)
	if err != nil {
		log.Fatalf("Error on DB connection %s", err.Error())
	}
	runningEvents, err := dbc.GetEvents(context.Background(), &pbc.GetEventRequest{Status: Running})
	if err != nil {
		log.Fatalf("Error on Getting events %s", err.Error())
	}
	var instanceConfig []store.InstanceConfig
	var challenges []store.Tag
	displayTimeFormat := "2006-01-02 15:04:05"
	alphaEvent := runningEvents.Events[0]
	startedAt, _ := time.Parse(displayTimeFormat, alphaEvent.StartedAt)
	finishedAt, _ := time.Parse(displayTimeFormat, alphaEvent.FinishedAt)
	listOfExercises := strings.Split(alphaEvent.Exercises, ",")
	instanceConfig = append(instanceConfig, store.InstanceConfig{
		Image:    "kali",
		MemoryMB: 4096,
		CPU:      1,
	})
	for _, e := range listOfExercises {
		challenges = append(challenges, store.Tag(e))
	}
	ts, _ := store.NewEventStore(store.EventConfig{
		Name:      alphaEvent.Name,
		Tag:       store.Tag(alphaEvent.Tag),
		Available: int(alphaEvent.Available),
		Capacity:  int(alphaEvent.Capacity),
		Lab: store.Lab{
			Exercises: challenges,
			Frontends: instanceConfig,
		},
		StartedAt:      &startedAt,
		FinishExpected: nil,
		FinishedAt:     &finishedAt,
	}, "events", dbc)

	ef, err := store.NewExerciseFile(configExercisePath)
	if err != nil {
		log.Println(err)
	}
	exer, err := ef.GetExercisesByTags(challenges...)
	if err != nil {
		log.Println("Get exercises by tags error: " + err.Error())
	}
	labConf := lab.Config{
		Exercises: exer,
	}
	var res []store.FlagConfig
	for _, exercise := range labConf.Exercises {
		res = append(res, exercise.Flags()...)
	}

	// Add the challenges to the teams
	// It not precise cause it tae the challenge main tag and not the children challenges
	for _, t := range ts.GetTeams() {
		for _, c := range exer {
			_, _ = t.AddChallenge(store.Challenge{
				Name:  c.Name,
				Tag:   c.Tags[0],
				Value: store.NewFlag().String(),
			})
		}
	}
	am := amigo.NewAmigo(ts, res)

	log.Fatal(http.ListenAndServe(":8080", am.Handler(nil, http.NewServeMux())))
}
