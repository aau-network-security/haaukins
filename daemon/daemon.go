package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/aau-network-security/go-ntp/event"
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	metadata "google.golang.org/grpc/metadata"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"

	"github.com/rs/zerolog/log"
)

var (
	DuplicateEventErr = errors.New("event with that tag already exists")
	UnknownEventErr   = errors.New("unable to find event by that tag")
	MissingTokenErr   = errors.New("No security token provided")
)

type daemon struct {
	conf            *Config
	uh              UserHub
	events          map[string]event.Event
	exerciseLib     *exercise.Library
	frontendLibrary vbox.Library
	mux             *mux.Router
}

func New(conf *Config) (*daemon, error) {
	if conf.Host == "" {
		conf.Host = "localhost"
	}

	if conf.OvaDir == "" {
		dir, _ := os.Getwd()
		conf.OvaDir = filepath.Join(dir, "vbox")
	}

	users := map[string]*User{}
	for i, _ := range conf.Users {
		u := conf.Users[i]
		users[u.Username] = &u
	}

	elib, err := exercise.NewLibrary("exercises.yml")
	if err != nil {
		return nil, err
	}

	vlib := vbox.NewLibrary(conf.OvaDir)
	m := mux.NewRouter()
	go func() {
		if err := http.ListenAndServe(":8080", m); err != nil {
			fmt.Println("Serving error", err)
		}
	}()

	d := &daemon{
		conf:            conf,
		uh:              NewUserHub(conf),
		events:          make(map[string]event.Event),
		exerciseLib:     elib,
		frontendLibrary: vlib,
		mux:             m,
	}

	return d, nil
}

func (d *daemon) authorize(ctx context.Context) error {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if len(md["token"]) > 0 {
			token := md["token"][0]

			return d.uh.AuthenticateUserByToken(token)
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
	token, err := d.uh.TokenForUser(req.Username, req.Password)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	return &pb.LoginUserResponse{Token: token}, nil
}

func (d *daemon) SignupUser(ctx context.Context, req *pb.SignupUserRequest) (*pb.LoginUserResponse, error) {
	k := SignupKey(req.Key)
	if err := d.uh.AddUser(k, req.Username, req.Password); err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	token, err := d.uh.TokenForUser(req.Username, req.Password)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	return &pb.LoginUserResponse{Token: token}, nil
}

func (d *daemon) InviteUser(ctx context.Context, req *pb.InviteUserRequest) (*pb.InviteUserResponse, error) {
	k, err := d.uh.CreateSignupKey()
	if err != nil {
		return &pb.InviteUserResponse{
			Error: err.Error(),
		}, nil
	}

	return &pb.InviteUserResponse{
		Key: string(k),
	}, nil
}

func (d *daemon) CreateEvent(req *pb.CreateEventRequest, resp pb.Daemon_CreateEventServer) error {
	_, ok := d.events[req.Tag]
	if ok {
		return DuplicateEventErr
	}

	var exer []exercise.Config
	var err error
	if len(req.Exercises) > 0 {
		exer, err = d.exerciseLib.GetByTags(req.Exercises[0], req.Exercises[1:]...)
		if err != nil {
			log.Error().Err(err).Msg("Could not get exercises by tags")
			return err
		}
	}

	if req.Buffer == 0 {
		req.Buffer = 2
	}

	if req.Capacity == 0 {
		req.Capacity = 10
	}

	ev, err := event.New(event.Config{
		Name:     req.Name,
		Tag:      req.Tag,
		Buffer:   int(req.Buffer),
		Capacity: int(req.Capacity),
		LabConfig: lab.LabConfig{
			Frontends: req.Frontends,
			Exercises: exer,
		},
		VBoxLibrary: d.frontendLibrary,
	})
	if err != nil {
		log.Error().Err(err).Msg("Error creating event")
		return err
	}

	go ev.Start(context.TODO())

	subdomain := fmt.Sprintf("%s.%s", req.Tag, d.conf.Host)
	eventRoute := d.mux.Host(subdomain).Subrouter()
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
	log.Debug().Msg("Listing events..")

	var events []*pb.ListEventsResponse_Events
	var eventConf event.Config
	var tempExer []string

	for _, event := range d.events {
		eventConf = event.GetConfig()

		for _, exercise := range eventConf.LabConfig.Exercises {
			tempExer = append(tempExer, exercise.Name)
		}

		events = append(events, &pb.ListEventsResponse_Events{
			Name:      eventConf.Name,
			Tag:       eventConf.Tag,
			Buffer:    int32(eventConf.Buffer),
			Capacity:  int32(eventConf.Capacity),
			Frontends: eventConf.LabConfig.Frontends,
			Exercises: tempExer,
		})
	}

	return &pb.ListEventsResponse{Events: events}, nil
}

func (d *daemon) ListEventGroups(ctx context.Context, req *pb.ListEventGroupsRequest) (*pb.ListEventGroupsResponse, error) {
	log.Debug().Msg("Listing event groups..")

	var eventGroups []*pb.ListEventGroupsResponse_Groups

	ev, ok := d.events[req.Tag]
	if !ok {
		return nil, UnknownEventErr
	}

	groups := ev.GetGroups()

	for _, group := range groups {
		eventGroups = append(eventGroups, &pb.ListEventGroupsResponse_Groups{
			Name:   group.Name,
			LabTag: group.Lab.GetTag(),
		})
	}

	return &pb.ListEventGroupsResponse{Groups: eventGroups}, nil
}

func (d *daemon) Close() {
	for t, ev := range d.events {
		ev.Close()
		delete(d.events, t)
	}
}
