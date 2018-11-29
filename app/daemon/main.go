package main

import (
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/aau-network-security/go-ntp/daemon"
	"google.golang.org/grpc/reflection"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	mngtPort          = ":5454"
	defaultConfigFile = "config.yml"
)

func handleCancel(clean func() error) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Info().Msgf("Shutting down gracefully...")
		if err := clean(); err != nil {
			log.Error().Msgf("Error while shutting down: %s", err)
			os.Exit(1)
		}
		log.Info().Msgf("Closed daemon")
		os.Exit(0)
	}()
}

func optsFromConf(c *daemon.Config) ([]grpc.ServerOption, error) {
	crt := c.Management.TLS.CertFile
	key := c.Management.TLS.KeyFile
	if crt != "" && key != "" {
		creds, err := credentials.NewServerTLSFromFile(crt, key)
		if err != nil {
			return nil, err
		}
		return []grpc.ServerOption{grpc.Creds(creds)}, nil
	}
	return []grpc.ServerOption{}, nil
}

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	confFilePtr := flag.String("config", defaultConfigFile, "configuration file")
	c, err := daemon.NewConfigFromFile(*confFilePtr)
	if err != nil {
		log.Fatal().
			Err(err).
			Str("file", *confFilePtr).
			Msgf("Unable to read configuration file")
		return
	}

	lis, err := net.Listen("tcp", mngtPort)
	if err != nil {
		log.Fatal().
			Err(err).
			Msgf("Failed to listen on management port %s", mngtPort)
		return
	}

	d, err := daemon.New(c)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Unable to create daemon")
		return
	}

	handleCancel(func() error {
		return d.Close()
	})
	log.Info().Msgf("Started daemon")

	opts, err := optsFromConf(c)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to retrieve server options")
		return
	}

	s := d.GetServer(opts...)
	pb.RegisterDaemonServer(s, d)

	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed to start accepting incoming connections")
	}
}
