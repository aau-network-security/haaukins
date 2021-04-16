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
	"time"

	"github.com/aau-network-security/haaukins/daemon"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
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

func handleHotConfigReload(confFile *string, reload func(confFile *string) error) {

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)
	go func() {
		<-c
		log.Info().Msgf("Hot reload for config file...")
		if err := reload(confFile); err != nil {
			log.Error().Msgf("Error on reloading config file: %s", err)
			os.Exit(1)
		}
		log.Info().Msgf("Config is updated !")
	}()
}

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	confFilePtr := flag.String("config", defaultConfigFile, "configuration file")
	flag.Parse()

	// ensure that gRPC port is free to allocate
	conn, err := net.DialTimeout("tcp", daemon.MngtPort, time.Second)
	if conn != nil {
		_ = conn.Close()
		fmt.Printf("Checking gRPC port %s report: %v\n", daemon.MngtPort, daemon.PortIsAllocatedError)
		return
	}

	c, err := daemon.NewConfigFromFile(*confFilePtr)
	if err != nil {
		fmt.Printf("unable to read configuration file \"%s\": %s\n", *confFilePtr, err)
		return
	}

	d, err := daemon.New(c)
	if err != nil {
		fmt.Printf("unable to create daemon: %s\n", err)
		return
	}

	handleCancel(func() error {
		return d.Close()
	})

	handleHotConfigReload(confFilePtr, func(confFile *string) error {
		return d.ReloadConfig(confFilePtr)
	})

	log.Info().Msgf("Started daemon")

	if err := d.Run(); err != nil {
		log.Fatal().Err(err).Msg("")
	}
}
