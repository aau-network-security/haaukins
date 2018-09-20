package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/aau-network-security/go-ntp/daemon"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	dockerclient "github.com/fsouza/go-dockerclient"
	"google.golang.org/grpc/reflection"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
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

	c, err := daemon.NewConfigFromFile("config.yml")
	if err != nil {
		log.Fatal().Err(err)
	}
	fmt.Println(c)

	lis, err := net.Listen("tcp", ":5454")
	if err != nil {
		log.Info().Msg("failed to listen")
	}

	d, err := daemon.New(c)
	if err != nil {
		log.Fatal().Err(err)
	}

	s := d.GetServer()
	pb.RegisterDaemonServer(s, d)
	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatal().Err(err)
	}
}
