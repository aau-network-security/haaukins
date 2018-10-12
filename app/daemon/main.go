package main

import (
	"crypto/tls"
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

func listenerFromConf(c *daemon.Config, port string) (net.Listener, error) {
	if c.TLS.Management.CertFile != "" && c.TLS.Management.KeyFile != "" {
		cer, err := tls.LoadX509KeyPair(
			c.TLS.Management.CertFile,
			c.TLS.Management.KeyFile,
		)
		if err != nil {
			return nil, err
		}

		config := &tls.Config{Certificates: []tls.Certificate{cer}}
		return tls.Listen("tcp", port, config)
	}

	return net.Listen("tcp", port)
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

	lis, err := listenerFromConf(c, mngtPort)
	if err != nil {
		log.Info().Msg("Failed to listen on management port: " + mngtPort)
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

	s := d.GetServer()
	pb.RegisterDaemonServer(s, d)

	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatal().Err(err)
	}

}
