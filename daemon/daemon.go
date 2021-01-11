// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package daemon

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"

	wg "github.com/aau-network-security/haaukins/network/vpn"
	eservice "github.com/aau-network-security/haaukins/store/eproto"

	"github.com/aau-network-security/haaukins/svcs/guacamole"

	pb "github.com/aau-network-security/haaukins/daemon/proto"
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
)

var (
	DuplicateEventErr    = errors.New("Event with that tag already exists")
	UnknownEventErr      = errors.New("Unable to find event by that tag")
	MissingTokenErr      = errors.New("No security token provided")
	InvalidArgumentsErr  = errors.New("Invalid arguments provided")
	UnknownTeamErr       = errors.New("Unable to find team by that id")
	GrpcOptsErr          = errors.New("failed to retrieve server options")
	NoLabByTeamIdErr     = errors.New("Lab is nil, no lab found for given team id ! ")
	PortIsAllocatedError = errors.New("Given gRPC port is already allocated")
	ReservedDomainErr    = errors.New("Reserved sub domain, change event tag !  ")

	ReservedSubDomains = map[string]bool{"docs": true, "admin": true, "grpc": true, "api": true, "vpn": true}
	version            string
	schedulers         []jobSpecs
)

const (
	MngtPort           = ":5454"
	displayTimeFormat  = time.RFC3339
	dbTimeFormat       = "2006-01-02 15:04:05"
	labCheckInterval   = 5 * time.Hour
	eventCheckInterval = 8 * time.Hour
	closeEventCI       = 12 * time.Hour
	Running            = int32(0)
	Suspended          = int32(1)
	Booked             = int32(2)
	Closed             = int32(3)
	SuspendTeamS       = "Suspend Team Scheduler"
	BookEventS         = "Check Booked Event Scheduler"
	CheckOverdueEventS = "Check Overdue Event Scheduler"
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

type jobSpecs struct {
	function      func() error
	checkInterval time.Duration
}

type daemon struct {
	conf  *Config
	auth  Authenticator
	users store.UsersFile
	//exercises store.ExerciseStore
	eventPool *eventPool
	frontends store.FrontendStore
	ehost     guacamole.Host
	logPool   logging.Pool
	closers   []io.Closer
	dbClient  pbc.StoreClient
	exClient  eservice.ExerciseStoreClient
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

	// todo: replace all if statement with something better
	// change the way of handling configuration files

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

	if c.ConfFiles.OvaDir == "" {
		dir, _ := os.Getwd()
		c.ConfFiles.OvaDir = filepath.Join(dir, "vbox")
	}

	if c.ConfFiles.LogDir == "" {
		dir, _ := os.Getwd()
		c.ConfFiles.LogDir = filepath.Join(dir, "logs")
	}

	if c.ConfFiles.UsersFile == "" {
		c.ConfFiles.UsersFile = "users.yml"
	}

	if c.ConfFiles.ExercisesFile == "" {
		c.ConfFiles.ExercisesFile = "exercises.yml"
	}

	if c.ConfFiles.FrontendsFile == "" {
		c.ConfFiles.FrontendsFile = "frontends.yml"
	}

	if c.ConfFiles.EventsDir == "" {
		c.ConfFiles.EventsDir = "events"
	}

	if c.Database.AuthKey == "" {
		log.Info().Str("DB AUTH KEY", "development-environment").
			Msg("Database authentication key set ")
		c.Database.AuthKey = "development-environment"
	}

	if c.Database.SignKey == "" {
		log.Info().Str("DB Signin KEY", "dev-env").
			Msg("Database authentication key set ")
		c.Database.SignKey = "dev-env"
	}

	if c.Database.Grpc == "" {
		log.Info().Str("DB GRPC Server", "localhost:50051").
			Msg("DB GRPC connection default value for dev environment ")
		c.Database.Grpc = "localhost:50051"
	}

	if c.Certs.Enabled {
		if c.Certs.Directory == "" {
			usr, err := user.Current()
			if err != nil {
				return nil, errors.New("Invalid user")
			}
			c.Certs.Directory = filepath.Join(usr.HomeDir, ".local", "share", "certmagic")
		}
	}

	return &c, nil
}

func New(conf *Config) (*daemon, error) {
	uf, err := store.NewUserFile(conf.ConfFiles.UsersFile)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to read users file: %s", conf.ConfFiles.UsersFile))
	}

	//ef, err := store.NewExerciseFile(conf.ConfFiles.ExercisesFile)
	//if err != nil {
	//	return nil, errors.Wrap(err, fmt.Sprintf("unable to read exercises file: %s", conf.ConfFiles.ExercisesFile))
	//}

	ff, err := store.NewFrontendsFile(conf.ConfFiles.FrontendsFile)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to read frontends file: %s", conf.ConfFiles.FrontendsFile))
	}

	vlib := vbox.NewLibrary(conf.ConfFiles.OvaDir)
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
	// todo: could be done in much better way.

	dbConfig := store.Config{
		Grpc:     conf.Database.Grpc,
		AuthKey:  conf.Database.AuthKey,
		SignKey:  conf.Database.SignKey,
		Enabled:  conf.Database.CertConfig.Enabled,
		CertFile: conf.Database.CertConfig.CertFile,
		CertKey:  conf.Database.CertConfig.CertKey,
		CAFile:   conf.Database.CertConfig.CAFile,
	}

	vpnConfig := wg.WireGuardConfig{
		Endpoint: conf.VPNConn.Endpoint,
		Port:     conf.VPNConn.Port,
		AuthKey:  conf.VPNConn.AuthKey,
		SignKey:  conf.VPNConn.SignKey,
		Enabled:  conf.VPNConn.CertConf.Enabled,
		CertFile: conf.VPNConn.CertConf.CertFile,
		CertKey:  conf.VPNConn.CertConf.CertKey,
		CAFile:   conf.VPNConn.CertConf.CAFile,
		Dir:      conf.VPNConn.Dir,
	}

	eserviceConfig := store.Config{
		Grpc:     conf.ExerciseService.Grpc,
		AuthKey:  conf.ExerciseService.AuthKey,
		SignKey:  conf.ExerciseService.SignKey,
		Enabled:  conf.ExerciseService.CertConfig.Enabled,
		CertFile: conf.ExerciseService.CertConfig.CertFile,
		CertKey:  conf.ExerciseService.CertConfig.CertKey,
		CAFile:   conf.ExerciseService.CertConfig.CAFile,
	}

	exServiceClient, err := store.NewExerciseClientConn(eserviceConfig)
	if err != nil {
		return nil, fmt.Errorf("[exercise-service]: error on creating gRPC communication %v ", err)
	}

	dbc, err := store.NewGRPClientDBConnection(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("[store-service]: error on creating GRPClient DB Connection %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// query running events only
	runningEvents, err := dbc.GetEvents(ctx, &pbc.GetEventRequest{Status: Running})
	if err != nil {
		log.Error().Msgf("error on getting events from database %v", err)
		return nil, store.TranslateRPCErr(err)
	}

	logPool, err := logging.NewPool(conf.ConfFiles.LogDir)
	if err != nil {
		return nil, fmt.Errorf("error on creating new pool for looging :  %v", err)
	}

	d := &daemon{
		conf:  conf,
		auth:  NewAuthenticator(uf, conf.SigningKey),
		users: uf,
		//exercises: ef,
		eventPool: eventPool,
		frontends: ff,
		ehost:     guacamole.NewHost(vlib, exServiceClient, conf.ConfFiles.EventsDir, dbc, vpnConfig),
		logPool:   logPool,
		closers:   []io.Closer{logPool, eventPool},
		dbClient:  dbc,
		exClient:  exServiceClient,
	}

	for _, ef := range runningEvents.Events {
		// check through status of event
		// suspended is also included since at first start
		// daemon should be aware of the event which is suspended
		// and configuration should be loaded to daemon
		if ef.Status == Running || ef.Status == Suspended {
			vpnIP, err := getVPNIP()
			if err != nil {
				log.Error().Msgf("Getting VPN address error on New() in daemon %v", err)
			}
			eventConfig := d.generateEventConfig(ef, ef.Status, vpnIP)
			err = d.createEventFromEventDB(context.Background(), eventConfig)
			if err != nil {
				return nil, fmt.Errorf("Error on creating event from db: %v", err)

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
	if d.conf.Certs.Enabled {
		// Load cert pairs
		certificate, err := tls.LoadX509KeyPair(d.conf.Certs.CertFile, d.conf.Certs.CertKey)
		if err != nil {
			return nil, fmt.Errorf("could not load server key pair: %s", err)
		}

		// Create a certificate pool from the certificate authority
		certPool := x509.NewCertPool()
		ca, err := ioutil.ReadFile(d.conf.Certs.CAFile)
		if err != nil {
			return nil, fmt.Errorf("HAAUKINS Grpc could not read ca certificate: %s", err)
		}
		// CA file for let's encrypt is located under domain conf as `chain.pem`
		// pass chain.pem location
		// Append the client certificates from the CA
		if ok := certPool.AppendCertsFromPEM(ca); !ok {
			return nil, errors.New("failed to append client certs")
		}

		// Create the TLS credentials
		creds := credentials.NewTLS(&tls.Config{
			// no need to RequireAndVerifyClientCert
			Certificates: []tls.Certificate{certificate},
			ClientCAs:    certPool,
		})

		return []grpc.ServerOption{grpc.Creds(creds)}, nil
	}
	return []grpc.ServerOption{}, nil
}

func (d *daemon) RunScheduler(job jobSpecs) error {
	timePeriod := job.checkInterval
	command := job.function
	ticker := time.NewTicker(timePeriod)

	var schedulerError error
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := command(); err != nil {
					schedulerError = err
				}
			}

		}

	}()

	return schedulerError
}

func (d *daemon) Run() error {

	// start frontend
	go func() {
		if d.conf.Certs.Enabled {
			if err := http.ListenAndServeTLS(fmt.Sprintf(":%d", d.conf.Port.Secure), d.conf.Certs.CertFile, d.conf.Certs.CertKey, d.eventPool); err != nil {
				log.Warn().Msgf("Serving error: %s", err)
			}
			return
		}
		if err := http.ListenAndServe(fmt.Sprintf(":%d", d.conf.Port.InSecure), d.eventPool); err != nil {
			log.Warn().Msgf("Serving error: %s", err)
		}
	}()
	// redirect if TLS enabled only...
	if d.conf.Certs.Enabled {
		go http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://"+r.Host+r.URL.String(), http.StatusMovedPermanently)
		}))
	}
	// start gRPC daemon
	lis, err := net.Listen("tcp", MngtPort)
	if err != nil {
		return &MngtPortErr{MngtPort}
	}
	log.Info().Msg("gRPC daemon has been started  ! on port :5454")

	opts, err := d.grpcOpts()
	if err != nil {
		return errors.Wrap(GrpcOptsErr, err.Error())
	}
	s := d.GetServer(opts...)
	pb.RegisterDaemonServer(s, d)

	reflection.Register(s)
	log.Info().Msg("Reflection Registration is called.... ")

	// initialize schedulers
	if err := d.initializeScheduler(); err != nil {
		return err
	}

	return s.Serve(lis)
}

// calculateTotalConsumption will add up all running events resources
func (d *daemon) isFree(sT, fT time.Time, capacity int32) (bool, error) {
	log.Printf("Running isFree function")
	ctx := context.Background()
	m, err := d.dbClient.GetTimeSeries(ctx, &pbc.EmptyRequest{})
	if err != nil {
		log.Printf("Error on calculating cost %v", err)
		return false, err
	}

	timeInterval := getDates(sT, fT)
	for _, time := range timeInterval {
		m.Timeseries[time.Format(dbTimeFormat)] += capacity
		if m.Timeseries[time.Format(dbTimeFormat)] > 90 {
			// todo: return number of possible vms by dates
			return false, errors.New("Not available resource to book/create the event!, you may choose different date range ")
		}
	}
	return true, nil
}

func daysInDates(sT, fT time.Time) int {
	days := fT.Sub(sT).Hours() / 24
	return int(math.Round(days))
}

func zeroTime(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0000, time.UTC)
}

func getDates(sT, fT time.Time) []time.Time {
	var dates []time.Time

	// zeroing hour:minute:second and nanosecond and setting zone to UTC
	sT = zeroTime(sT)
	fT = zeroTime(fT)
	// calculate # of days in between dates
	days := daysInDates(sT, fT)
	var count int
	for count <= days {
		date := sT
		sT = date.AddDate(0, 0, 1)
		dates = append(dates, date)
		count++
	}
	return dates
}

func (d *daemon) initializeScheduler() error {
	jobs := make(map[string]jobSpecs)
	jobs[SuspendTeamS] = jobSpecs{
		function:      d.suspendTeams,
		checkInterval: labCheckInterval,
	}
	jobs[BookEventS] = jobSpecs{
		function:      d.visitBookedEvents,
		checkInterval: eventCheckInterval,
	}
	jobs[CheckOverdueEventS] = jobSpecs{
		function:      d.closeEvents,
		checkInterval: closeEventCI,
	}

	for name, job := range jobs {
		log.Info().Msgf("Running scheduler %s", name)
		if err := d.RunScheduler(job); err != nil {
			log.Error().Msgf("Error in scheduler with name %s and error %v ", name, err)
			return err
		}
	}
	return nil
}
