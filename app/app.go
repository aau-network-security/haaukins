package main

import (
	"context"
	"github.com/aau-network-security/go-ntp/api"
	"github.com/aau-network-security/go-ntp/event"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"os/signal"
	"syscall"
)

func handleCancel(clean func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		clean()
		os.Exit(1)
	}()
}

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	ev, err := event.New("app/config.yml", "app/exercises.yml")
	if err != nil {
		log.Error().Msgf("%s", err)
		return
	}
	handleCancel(func() {
		ev.Close()
		log.Info().Msgf("Closed event")
	})
	log.Info().Msgf("Created event")

	err = ev.Start(context.TODO())
	if err != nil {
		log.Error().Msgf("%s", err)
		ev.Close()
		return
	}
	log.Info().Msgf("Started event")
	api := api.Api{Event: ev}
	api.RunServer("localhost", 8080)
}
