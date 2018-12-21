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

	"crypto/tls"
	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	"github.com/mholt/certmagic"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"path/filepath"
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

func loadFromCertmagic(c *daemon.Config, ext string) ([]byte, error) {
	ca := certmagic.LetsEncryptProductionCA
	if c.TLS.ACME.Development {
		ca = certmagic.LetsEncryptStagingCA
	}

	kb := certmagic.KeyBuilder{}
	dir := filepath.Join(kb.SitePrefix(ca, c.Host.Grpc), c.Host.Grpc)

	key := fmt.Sprintf("%s.%s", dir, ext)
	if err := certmagic.DefaultStorage.Lock(key); err != nil {
		return nil, err
	}
	defer certmagic.DefaultStorage.Unlock(key)

	return certmagic.DefaultStorage.Load(key)
}

func loadCert(c *daemon.Config) ([]byte, error) {
	return loadFromCertmagic(c, "crt")
}

func loadKey(c *daemon.Config) ([]byte, error) {
	return loadFromCertmagic(c, "key")
}

func optsFromConf(c *daemon.Config) ([]grpc.ServerOption, error) {
	if c.TLS.Enabled {
		crtBytes, err := loadCert(c)
		if err != nil {
			return nil, err
		}
		keyBytes, err := loadKey(c)
		if err != nil {
			return nil, err
		}

		cert, err := tls.X509KeyPair(crtBytes, keyBytes)
		if err != nil {
			return nil, err
		}

		creds := credentials.NewServerTLSFromCert(&cert)
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
