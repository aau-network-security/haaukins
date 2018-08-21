package main

import (
	"context"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"

	"github.com/aau-network-security/go-ntp/api"
	"github.com/aau-network-security/go-ntp/event"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

func loadCredentials(path string) (*dockerclient.AuthConfiguration, error) {
	var authConfig *dockerclient.AuthConfiguration
	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(rawData, &authConfig); err != nil {
		return nil, err
	}

	return authConfig, nil
}

func handleCancel(clean func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Info().Msgf("Received SIGINT or SIGTERM: shutting down gracefully")
		clean()
		os.Exit(1)
	}()
}

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	authConfig, err := loadCredentials("auth.json")
	if err != nil {
		log.Info().Msgf("No registry credentials file found: %s", err)
	} else {
		docker.Registries[authConfig.ServerAddress] = *authConfig
	}

	ev, err := event.New("app/config.yml")
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

	hostIp, err := docker.GetDockerHostIP()
	if err != nil {
		log.Error().Msgf("Error while getting host IP: %s", err)
		return
	}

	log.Info().Msgf("Started event")
	api := api.Api{Event: ev}
	api.RunServer(hostIp, 3331)
}
