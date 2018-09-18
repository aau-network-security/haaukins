package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/aau-network-security/go-ntp/api"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	yaml "gopkg.in/yaml.v2"
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

	authConfig, err := loadCredentials("./auth.yml")
	if err != nil {
		log.Info().Msgf("No registry credentials file found: %s", err)
	} else {
		docker.Registries[authConfig.ServerAddress] = *authConfig
	}

	srv, err := api.NewServer(api.Config{
		Host: "localhost",
	})
	if err != nil {
		log.Fatal().Err(err)
	}

	handleCancel(func() {
		srv.Close()
		log.Info().Msgf("Closed server")
	})

	http.ListenAndServe(":5454", srv.Routes())
}
