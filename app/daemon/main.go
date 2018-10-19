package main

import (
	"flag"
	"fmt"
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

func handleCancel(clean func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Info().Msgf("Shutting down gracefully...")
		clean()
		os.Exit(1)
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
		fmt.Printf("unable to read configuration file \"%s\": %s\n", *confFilePtr, err)
		return
	}

	lis, err := net.Listen("tcp", mngtPort)
	if err != nil {
		log.Fatal().
			Err(err).
			Msgf("failed to listen on management port %s", mngtPort)
	}

	d, err := daemon.New(c)
	if err != nil {
		fmt.Printf("unable to create daemon: %s\n", err)
		return
	}

	handleCancel(func() {
		d.Close()
		log.Info().Msgf("Closed daemon")
	})
	log.Info().Msgf("Started daemon")

	opts, err := optsFromConf(c)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("failed to retrieve server options")
	}

	s := d.GetServer(opts...)
	pb.RegisterDaemonServer(s, d)

	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatal().Err(err)
	}

}
