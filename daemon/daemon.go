package daemon

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aau-network-security/go-ntp/event"
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	metadata "google.golang.org/grpc/metadata"
	yaml "gopkg.in/yaml.v2"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	dockerclient "github.com/fsouza/go-dockerclient"

	"github.com/rs/zerolog/log"
)

var (
	DuplicateEventErr   = errors.New("event with that tag already exists")
	UnknownEventErr     = errors.New("unable to find event by that tag")
	MissingTokenErr     = errors.New("no security token provided")
	InvalidArgumentsErr = errors.New("invalid arguments provided")
	MissingSecretKey    = errors.New("management signing key cannot be empty")
)

type Config struct {
	Host                 string                           `yaml:"host"`
	ManagementSigningKey string                           `yaml:"management-sign-key"`
	UsersFile            string                           `yaml:"users-file"`
	ExercisesFile        string                           `yaml:"exercises-file"`
	OvaDir               string                           `yaml:"ova-directory"`
	DockerRepositories   []dockerclient.AuthConfiguration `yaml:"docker-repositories,omitempty"`
	TLS                  struct {
		Management struct {
			CertFile string `yaml:"cert-file"`
			KeyFile  string `yaml:"key-file"`
		} `yaml:"management"`
	} `yaml:"tls,omitempty"`
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

	return &c, nil
}

type daemon struct {
	conf            *Config
	auth            Authenticator
	users           store.UsersFile
	exercises       store.ExerciseStore
	events          map[string]event.Event
	frontendLibrary vbox.Library
	mux             *mux.Router
	eh              event.Host
}

func New(conf *Config) (*daemon, error) {
	if conf.ManagementSigningKey == "" {
		return nil, MissingSecretKey
	}

	if conf.Host == "" {
		conf.Host = "localhost"
	}

	if conf.OvaDir == "" {
		dir, _ := os.Getwd()
		conf.OvaDir = filepath.Join(dir, "vbox")
	}

	if conf.UsersFile == "" {
		conf.UsersFile = "users.yml"
	}

	if conf.ExercisesFile == "" {
		conf.ExercisesFile = "exercises.yml"
	}

	uf, err := store.NewUserFile(conf.UsersFile)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to read users file: %s", conf.UsersFile))
	}

	ef, err := store.NewExerciseFile(conf.ExercisesFile)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to read exercises file: %s", conf.ExercisesFile))
	}

	vlib := vbox.NewLibrary(conf.OvaDir)
	m := mux.NewRouter()
	go func() {
		if err := http.ListenAndServe(":8080", m); err != nil {
			fmt.Println("Serving error", err)
		}
	}()

	d := &daemon{
		conf:      conf,
		auth:      NewAuthenticator(uf, conf.ManagementSigningKey),
		users:     uf,
		exercises: ef,

		events:          make(map[string]event.Event),
		frontendLibrary: vlib,
		mux:             m,
		eh:              event.NewHost(vlib, ef),
	}

	return d, nil
}

func (d *daemon) authorize(ctx context.Context) error {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if len(md["token"]) > 0 {
			token := md["token"][0]

			return d.auth.AuthenticateUserByToken(token)
		}

		return MissingTokenErr
	}

	return nil
}

func (d *daemon) GetServer() *grpc.Server {
	nonAuth := []string{"LoginUser", "SignupUser"}

	streamInterceptor := func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		for _, endpoint := range nonAuth {
			if strings.HasSuffix(info.FullMethod, endpoint) {
				return handler(srv, stream)
			}
		}

		if err := d.authorize(stream.Context()); err != nil {
			return err
		}

		return handler(srv, stream)
	}

	unaryInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		for _, endpoint := range nonAuth {
			if strings.HasSuffix(info.FullMethod, endpoint) {
				return handler(ctx, req)
			}
		}

		if err := d.authorize(ctx); err != nil {
			return nil, err
		}

		return handler(ctx, req)
	}

	return grpc.NewServer(
		grpc.StreamInterceptor(streamInterceptor),
		grpc.UnaryInterceptor(unaryInterceptor),
	)
}

func (d *daemon) LoginUser(ctx context.Context, req *pb.LoginUserRequest) (*pb.LoginUserResponse, error) {
	token, err := d.auth.TokenForUser(req.Username, req.Password)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	return &pb.LoginUserResponse{Token: token}, nil
}

func (d *daemon) SignupUser(ctx context.Context, req *pb.SignupUserRequest) (*pb.LoginUserResponse, error) {
	u, err := store.NewUser(req.Username, req.Password)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	k := store.SignupKey(req.Key)
	if err := d.users.DeleteSignupKey(k); err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	if err := d.users.CreateUser(u); err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	token, err := d.auth.TokenForUser(req.Username, req.Password)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	return &pb.LoginUserResponse{Token: token}, nil
}

func (d *daemon) InviteUser(ctx context.Context, req *pb.InviteUserRequest) (*pb.InviteUserResponse, error) {
	k := store.NewSignupKey()

	if err := d.users.CreateSignupKey(k); err != nil {
		return &pb.InviteUserResponse{
			Error: err.Error(),
		}, nil
	}

	return &pb.InviteUserResponse{
		Key: string(k),
	}, nil
}

func (d *daemon) CreateEvent(req *pb.CreateEventRequest, resp pb.Daemon_CreateEventServer) error {
	log.Info().
		Str("Name", req.Name).
		Str("Tag", req.Tag).
		Int32("Buffer", req.Buffer).
		Int32("Capacity", req.Capacity).
		Strs("Exercises", req.Exercises).
		Strs("Frontends", req.Frontends).
		Str("Cap", req.Tag).
		Msg("Creating event")

	if req.Name == "" || req.Tag == "" {
		return InvalidArgumentsErr
	}

	_, ok := d.events[req.Tag]
	if ok {
		return DuplicateEventErr
	}

	if req.Buffer == 0 {
		req.Buffer = 2
	}

	if req.Capacity == 0 {
		req.Capacity = 10
	}

	tags := make([]exercise.Tag, len(req.Exercises))
	for i, s := range req.Exercises {
		t, err := exercise.NewTag(s)
		if err != nil {
			return err
		}
		tags[i] = t
	}

	ev, err := d.eh.CreateEvent(store.Event{
		Name:     req.Name,
		Tag:      req.Tag,
		Buffer:   int(req.Buffer),
		Capacity: int(req.Capacity),
		Lab: store.Lab{
			Frontends: req.Frontends,
			Exercises: tags,
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("Error creating event")
		return err
	}

	go ev.Start(context.TODO())

	host := fmt.Sprintf("%s.%s", req.Tag, d.conf.Host)
	eventRoute := d.mux.Host(host).Subrouter()
	ev.Connect(eventRoute)

	d.events[req.Tag] = ev

	return nil
}

func (d *daemon) StopEvent(req *pb.StopEventRequest, resp pb.Daemon_StopEventServer) error {
	ev, ok := d.events[req.Tag]
	if !ok {
		return UnknownEventErr
	}

	delete(d.events, req.Tag)

	ev.Close()
	return nil
}

func (d *daemon) RestartGroupLab(req *pb.RestartGroupLabRequest, resp pb.Daemon_RestartGroupLabServer) error {
	ev, ok := d.events[req.EventTag]
	if !ok {
		return UnknownEventErr
	}

	lab, err := ev.GetHub().GetLabByTag(req.LabTag)

	if err != nil {
		return err
	}

	if err := lab.Restart(); err != nil {
		return err
	}

	return nil
}

func (d *daemon) ListEvents(ctx context.Context, req *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	// log.Debug().Msg("Listing events..")

	// var events []*pb.ListEventsResponse_Events
	// var eventConf store.Event
	// var tempExer []string

	// for _, event := range d.events {
	// 	eventConf = event.GetConfig()

	// 	for _, exercise := range eventConf.LabConfig.Exercises {
	// 		tempExer = append(tempExer, exercise.Name)
	// 	}

	// 	events = append(events, &pb.ListEventsResponse_Events{
	// 		Name:      eventConf.Name,
	// 		Tag:       eventConf.Tag,
	// 		Buffer:    int32(eventConf.Buffer),
	// 		Capacity:  int32(eventConf.Capacity),
	// 		Frontends: eventConf.LabConfig.Frontends,
	// 		Exercises: tempExer,
	// 	})
	// }

	var events []*pb.ListEventsResponse_Events

	return &pb.ListEventsResponse{Events: events}, nil
}

func (d *daemon) ListEventGroups(ctx context.Context, req *pb.ListEventGroupsRequest) (*pb.ListEventGroupsResponse, error) {
	log.Debug().Msg("Listing event groups..")

	var eventGroups []*pb.ListEventGroupsResponse_Groups

	// ev, ok := d.events[req.Tag]
	// if !ok {
	// 	return nil, UnknownEventErr
	// }

	// groups := ev.GetGroups()

	// for _, group := range groups {
	// 	eventGroups = append(eventGroups, &pb.ListEventGroupsResponse_Groups{
	// 		Name:   group.Name,
	// 		LabTag: group.Lab.GetTag(),
	// 	})
	// }

	return &pb.ListEventGroupsResponse{Groups: eventGroups}, nil
}

func (d *daemon) Close() {
	for t, ev := range d.events {
		ev.Close()
		delete(d.events, t)
	}
}
