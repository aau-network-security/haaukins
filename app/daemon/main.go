// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/aau-network-security/haaukins/daemon"
	"google.golang.org/grpc/reflection"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/mholt/certmagic"
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
	if c.TLS.Enabled {
		domains := []string{
			c.Host.Grpc,
		}

		err := certmagic.Manage(domains)
		if err != nil {
			return nil, err
		}
		cert, err := certmagic.Default.CacheManagedCertificate(c.Host.Grpc)
		if err != nil {
			return nil, err
		}
		creds := credentials.NewServerTLSFromCert(&cert.Certificate)
		return []grpc.ServerOption{grpc.Creds(creds)}, nil
	}
	return []grpc.ServerOption{}, nil
}

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	confFilePtr := flag.String("config", defaultConfigFile, "configuration file")
	flag.Parse()

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

	handleCancel(func() error {
		return d.Close()
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
