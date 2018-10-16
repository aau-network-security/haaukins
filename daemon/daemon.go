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
	DuplicateEventErr   = errors.New("Event with that tag already exists")
	UnknownEventErr     = errors.New("Unable to find event by that tag")
	MissingTokenErr     = errors.New("No security token provided")
	InvalidArgumentsErr = errors.New("Invalid arguments provided")
	MissingSecretKey    = errors.New("Management signing key cannot be empty")

	version string
)

type Config struct {
	Host               string                           `yaml:"host,omitempty"`
	Port               uint                             `yaml:"port,omitempty"`
	UsersFile          string                           `yaml:"users-file,omitempty"`
	ExercisesFile      string                           `yaml:"exercises-file,omitempty"`
	OvaDir             string                           `yaml:"ova-directory,omitempty"`
	EventsDir          string                           `yaml:"events-directory,omitempty"`
	DockerRepositories []dockerclient.AuthConfiguration `yaml:"docker-repositories,omitempty"`
	Management         struct {
		SigningKey string `yaml:"sign-key"`
		TLS        struct {
			CertFile string `yaml:"cert-file"`
			KeyFile  string `yaml:"key-file"`
		} `yaml:"TLS"`
	} `yaml:"management,omitempty"`
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
	events          map[store.Tag]event.Event
	frontendLibrary vbox.Library
	mux             *mux.Router
	ehost           event.Host
}

func New(conf *Config) (*daemon, error) {
	if conf.Management.SigningKey == "" {
		return nil, MissingSecretKey
	}

	if conf.Host == "" {
		conf.Host = "localhost"
	}

	if conf.Port == 0 {
		conf.Port = 80
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

	if conf.EventsDir == "" {
		conf.EventsDir = "events"
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
		if err := http.ListenAndServe(fmt.Sprintf(":%d", conf.Port), m); err != nil {
			fmt.Println("Serving error", err)
		}
	}()

	if len(uf.ListUsers()) == 0 && len(uf.ListSignupKeys()) == 0 {
		k := store.NewSignupKey()
		if err := uf.CreateSignupKey(k); err != nil {
			return nil, err
		}

		log.Info().Msg("No users or signup keys found, creating a key")
	}

	keys := uf.ListSignupKeys()
	if len(uf.ListUsers()) == 0 && len(keys) > 0 {
		log.Info().Msg("No users found, printing keys")
		for _, k := range keys {
			log.Info().Str("key", string(k)).Msg("Found key")
		}
	}

	efh, err := store.NewEventFileHub(conf.EventsDir)
	if err != nil {
		return nil, err
	}

	d := &daemon{
		conf:            conf,
		auth:            NewAuthenticator(uf, conf.Management.SigningKey),
		users:           uf,
		exercises:       ef,
		events:          make(map[store.Tag]event.Event),
		frontendLibrary: vlib,
		mux:             m,
		ehost:           event.NewHost(vlib, ef, efh),
	}

	eventFiles, err := efh.GetUnfinishedEvents()
	if err != nil {
		return nil, err
	}

	for _, ef := range eventFiles {
		err := d.createEventFromEventFile(ef)
		if err != nil {
			return nil, err
		}
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

func (d *daemon) createEventFromEventFile(ef store.EventFile) error {
	ev, err := d.ehost.CreateEventFromEventFile(ef)
	if err != nil {
		log.Error().Err(err).Msg("Error creating event")
		return err
	}

	return d.createEvent(ev)
}

func (d *daemon) createEventFromConfig(conf store.EventConfig) error {
	ev, err := d.ehost.CreateEventFromConfig(conf)
	if err != nil {
		log.Error().Err(err).Msg("Error creating event")
		return err
	}

	return d.createEvent(ev)
}

func (d *daemon) createEvent(ev event.Event) error {
	conf := ev.GetConfig()
	log.Info().
		Str("Name", conf.Name).
		Str("Tag", string(conf.Tag)).
		Int("Buffer", conf.Buffer).
		Int("Capacity", conf.Capacity).
		Strs("Frontends", conf.Lab.Frontends).
		Msg("Creating event")

	go ev.Start(context.TODO())

	host := fmt.Sprintf("%s.%s", conf.Tag, d.conf.Host)
	eventRoute := d.mux.Host(host).Subrouter()
	ev.Connect(eventRoute)

	d.events[conf.Tag] = ev

	return nil
}

func (d *daemon) CreateEvent(req *pb.CreateEventRequest, resp pb.Daemon_CreateEventServer) error {
	tags := make([]store.Tag, len(req.Exercises))
	for i, s := range req.Exercises {
		t, err := store.NewTag(s)
		if err != nil {
			return err
		}
		tags[i] = t
	}

	evtag, _ := store.NewTag(req.Tag)
	conf := store.EventConfig{
		Name:     req.Name,
		Tag:      evtag,
		Buffer:   int(req.Buffer),
		Capacity: int(req.Capacity),
		Lab: store.Lab{
			Frontends: req.Frontends,
			Exercises: tags,
		},
	}

	if err := conf.Validate(); err != nil {
		return err
	}

	_, ok := d.events[evtag]
	if ok {
		return DuplicateEventErr
	}

	if conf.Buffer == 0 {
		conf.Buffer = 2
	}

	if conf.Capacity == 0 {
		conf.Capacity = 10
	}

	return d.createEventFromConfig(conf)
}

func (d *daemon) StopEvent(req *pb.StopEventRequest, resp pb.Daemon_StopEventServer) error {
	evtag, err := store.NewTag(req.Tag)
	if err != nil {
		return err
	}

	ev, ok := d.events[evtag]
	if !ok {
		return UnknownEventErr
	}

	delete(d.events, evtag)

	ev.Close()
	ev.Finish()
	return nil
}

func (d *daemon) RestartTeamLab(req *pb.RestartTeamLabRequest, resp pb.Daemon_RestartTeamLabServer) error {
	evtag, err := store.NewTag(req.EventTag)
	if err != nil {
		return err
	}

	ev, ok := d.events[evtag]
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

func (d *daemon) ResetExercise(req *pb.ResetExerciseRequest, resp pb.Daemon_ResetExerciseServer) error {
	evtag, err := store.NewTag(req.EventTag)
	if err != nil {
		return err
	}
	ev, ok := d.events[evtag]
	if !ok {
		return UnknownEventErr
	}

	if req.Teams != nil {
		// the requests has a selection of group ids
		for _, reqTeam := range req.Teams {
			if lab, ok := ev.GetLabByTeam(reqTeam.Id); ok {
				if err := lab.GetEnvironment().ResetByTag(req.ExerciseTag); err != nil {
					return err
				}
				resp.Send(&pb.ResetExerciseStatus{TeamId: reqTeam.Id})
			}
		}
	} else {
		// all exercises should be reset
		for _, t := range ev.GetTeams() {
			lab, _ := ev.GetLabByTeam(t.Id)
			if err := lab.GetEnvironment().ResetByTag(req.ExerciseTag); err != nil {
				return err
			}
			resp.Send(&pb.ResetExerciseStatus{TeamId: t.Id})
		}
	}
	return nil
}

func (d *daemon) ListEvents(ctx context.Context, req *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	var events []*pb.ListEventsResponse_Events

	for _, event := range d.events {
		conf := event.GetConfig()

		events = append(events, &pb.ListEventsResponse_Events{
			Name:          conf.Name,
			Tag:           string(conf.Tag),
			TeamCount:     int32(len(event.GetTeams())),
			ExerciseCount: int32(len(conf.Lab.Exercises)),
			Capacity:      int32(conf.Capacity),
		})
	}

	return &pb.ListEventsResponse{Events: events}, nil
}

func (d *daemon) ListEventTeams(ctx context.Context, req *pb.ListEventTeamsRequest) (*pb.ListEventTeamsResponse, error) {
	var eventTeams []*pb.ListEventTeamsResponse_Teams
	evtag, err := store.NewTag(req.Tag)
	if err != nil {
		return nil, err
	}
	ev, ok := d.events[evtag]
	if !ok {
		return nil, UnknownEventErr
	}

	teams := ev.GetTeams()

	for _, t := range teams {
		eventTeams = append(eventTeams, &pb.ListEventTeamsResponse_Teams{
			Id:    t.Id,
			Name:  t.Name,
			Email: t.Email,
		})
	}

	return &pb.ListEventTeamsResponse{Teams: eventTeams}, nil
}

func (d *daemon) Close() {
	for t, ev := range d.events {
		ev.Close()
		delete(d.events, t)
	}
}

func (d *daemon) Version(context.Context, *pb.Empty) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{Version: version}, nil
}
