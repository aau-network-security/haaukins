// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"fmt"
	"github.com/aau-network-security/haaukins/api"
	pb "github.com/aau-network-security/haaukins/daemon/proto"
	"github.com/aau-network-security/haaukins/event"
	"github.com/aau-network-security/haaukins/logging"
	"github.com/aau-network-security/haaukins/store"
	pbc "github.com/aau-network-security/haaukins/store/proto"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"
)

var (
	DuplicateEventErr   = errors.New("Event with that tag already exists")
	UnknownEventErr     = errors.New("Unable to find event by that tag")
	MissingTokenErr     = errors.New("No security token provided")
	InvalidArgumentsErr = errors.New("Invalid arguments provided")
	UnknownTeamErr      = errors.New("Unable to find team by that id")
	GrpcOptsErr         = errors.New("failed to retrieve server options")
  	NoLabByTeamIdErr    = errors.New("Lab is nil, no lab found for given team id ! ")
	
  version string
)

const (
	mngtPort          = ":5454"
	displayTimeFormat = "2006-01-02 15:04:05"
)

type MissingConfigErr struct {
	Option string
}

type MngtPortErr struct {
	port string
}

type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

type GrpcLogger struct {
	resp pb.Daemon_CreateEventServer
}

type daemon struct {
	conf      *Config
	auth      Authenticator
	users     store.UsersFile
	exercises store.ExerciseStore
	eventPool *eventPool
	frontends store.FrontendStore
	ehost     event.Host
	logPool   logging.Pool
	closers   []io.Closer
}

func (m *MissingConfigErr) Error() string {
	return fmt.Sprintf("%s cannot be empty", m.Option)
}



func (m *MngtPortErr) Error() string {
	return fmt.Sprintf("failed to listen on management port %s", m.port)
}


func NewConfigFromFile(path string) (*Config, error) {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var c Config
	err = yaml.Unmarshal(f, &c)
	if err != nil {
		return nil, err
	}

	for _, repo := range c.DockerRepositories {
		docker.Registries[repo.ServerAddress] = repo
	}

	if c.SigningKey == "" {
		return nil, &MissingConfigErr{"Management signing key"}
	}

	if c.Host.Http == "" {
		c.Host.Http = "localhost"
	}

	if c.Host.Grpc == "" {
		c.Host.Grpc = "localhost"
	}

	if c.Port.InSecure == 0 {
		c.Port.InSecure = 80
	}

	if c.Port.Secure == 0 {
		c.Port.Secure = 443
	}

	if c.OvaDir == "" {
		dir, _ := os.Getwd()
		c.OvaDir = filepath.Join(dir, "vbox")
	}

	if c.LogDir == "" {
		dir, _ := os.Getwd()
		c.LogDir = filepath.Join(dir, "logs")
	}

	if c.UsersFile == "" {
		c.UsersFile = "users.yml"
	}

	if c.ExercisesFile == "" {
		c.ExercisesFile = "exercises.yml"
	}

	if c.FrontendsFile == "" {
		c.FrontendsFile = "frontends.yml"
	}

	if c.EventsDir == "" {
		c.EventsDir = "events"
	}

	if c.TLS.Enabled {
		if c.TLS.Directory == "" {
			usr, err := user.Current()
			if err != nil {
				return nil, errors.New("Invalid user")
			}
			c.TLS.Directory = filepath.Join(usr.HomeDir, ".local", "share", "certmagic")
		}
	}

	return &c, nil
}


func New(conf *Config) (*daemon, error) {
	uf, err := store.NewUserFile(conf.UsersFile)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to read users file: %s", conf.UsersFile))
	}

	ef, err := store.NewExerciseFile(conf.ExercisesFile)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to read exercises file: %s", conf.ExercisesFile))
	}

	ff, err := store.NewFrontendsFile(conf.FrontendsFile)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to read frontends file: %s", conf.FrontendsFile))
	}

	vlib := vbox.NewLibrary(conf.OvaDir)
	eventPool := NewEventPool(conf.Host.Http)

	if len(uf.ListUsers()) == 0 && len(uf.ListSignupKeys()) == 0 {
		k := store.NewSignupKey()
		k.WillBeSuperUser = true

		if err := uf.CreateSignupKey(k); err != nil {
			return nil, err
		}

		log.Info().Msg("No users or signup keys found, creating a key")
	}

	keys := uf.ListSignupKeys()
	if len(uf.ListUsers()) == 0 && len(keys) > 0 {
		log.Info().Msg("No users found, printing keys")
		for _, k := range keys {
			log.Info().Str("key", k.String()).Msg("Found key")
		}
	}

	dbConn := store.DBConn{
		Server:   conf.DBServer,
		CertFile: conf.TLS.CertFile,
		Tls:      conf.TLS.Enabled,
		AuthKey:  conf.AuthKey,
		SignKey:  conf.SignKey,
	}
	dbc, err := store.NewGRPClientDBConnection(dbConn)
	if err != nil {
		return nil, err
	}

	logPool, err := logging.NewPool(conf.LogDir)
	if err != nil {
		return nil, err
	}

	d := &daemon{
		conf:      conf,
		auth:      NewAuthenticator(uf, conf.SigningKey),
		users:     uf,
		exercises: ef,
		eventPool: eventPool,
		frontends: ff,
		ehost:     event.NewHost(vlib, ef, conf.EventsDir, dbc),
		logPool:   logPool,
		closers:   []io.Closer{logPool, eventPool},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var instanceConfig []store.InstanceConfig
	var exercises  []store.Tag
	eventsFromDB, err := dbc.GetEvents(ctx, &pbc.EmptyRequest{})

	if err != nil {
		return nil, store.TranslateRPCErr(err)
	}
	for _, ef := range eventsFromDB.Events {
		if ef.FinishedAt == "" { //check if the event is finished or not
			startedAt, _ := time.Parse(displayTimeFormat,ef.StartedAt)
			finishedAt, _ := time.Parse(displayTimeFormat,ef.FinishedAt)
			listOfExercises := strings.Split(ef.Exercises,",")
			instanceConfig = append(instanceConfig, ff.GetFrontends(ef.Frontends)[0])
			for _, e := range listOfExercises {
				exercises = append(exercises,store.Tag(e) )
			}
			eventConfig := store.EventConfig{
				Name:           ef.Name,
				Tag:            store.Tag(ef.Tag),
				Available:      int(ef.Available),
				Capacity:       int(ef.Capacity),
				Lab:            store.Lab{
					Frontends: instanceConfig ,
					Exercises: exercises ,
				},
				StartedAt:      &startedAt,
				FinishExpected: nil,
				FinishedAt:     &finishedAt,
			}
			err := d.createEventFromEventDB(context.Background(),eventConfig)
			if err != nil {
				return nil, err
			}
		}
	}

	return d, nil
}


func (l *GrpcLogger) Msg(msg string) error {
	s := pb.LabStatus{
		ErrorMessage: msg,
	}
	return l.resp.Send(&s)
}

func (d *daemon) Version(context.Context, *pb.Empty) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{Version: version}, nil
}

func (d *daemon) grpcOpts() ([]grpc.ServerOption, error) {
	if d.conf.TLS.Enabled {
		creds,err := credentials.NewServerTLSFromFile(d.conf.TLS.CertFile,d.conf.TLS.CertKey)
		if err !=nil {
			log.Error().Msgf("Error reading certificate from file %s ",err)
		}
		return []grpc.ServerOption{grpc.Creds(creds)}, nil
	}
	return []grpc.ServerOption{}, nil
}

func (d *daemon) Run() error {

	// start frontend
	go func() {
		if d.conf.TLS.Enabled {
			if err := http.ListenAndServeTLS(fmt.Sprintf(":%d", d.conf.Port.Secure),d.conf.TLS.CertFile,d.conf.TLS.CertKey,d.eventPool); err != nil {
				log.Warn().Msgf("Serving error: %s", err)
			}
			return
		}
		if err := http.ListenAndServe(fmt.Sprintf(":%d", d.conf.Port.InSecure), d.eventPool); err != nil {
			log.Warn().Msgf("Serving error: %s", err)
		}
	}()
	// redirect if TLS enabled only...
	if d.conf.TLS.Enabled {
		go http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://"+r.Host+r.URL.String(), http.StatusMovedPermanently)
		}))
	}
	// start gRPC daemon
	lis, err := net.Listen("tcp", mngtPort)
	if err != nil {
		return &MngtPortErr{mngtPort}
	}
	log.Info().Msg("gRPC daemon has been started  ! on port :5454")

	opts, err := d.grpcOpts()
	if err != nil {
		return GrpcOptsErr
	}
	s := d.GetServer(opts...)
	pb.RegisterDaemonServer(s, d)

	reflection.Register(s)
	log.Info().Msg("Reflection Registration is called.... ")


	learningMaterialAPI := api.NewLearningMaterialAPI()

	http.ListenAndServe(":8888", learningMaterialAPI.Handler())

	return s.Serve(lis)
}
