package daemon

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aau-network-security/go-ntp/event"
	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	yaml "gopkg.in/yaml.v2"

	pb "github.com/aau-network-security/go-ntp/daemon/proto"
	dockerclient "github.com/fsouza/go-dockerclient"

	"sync"

	"github.com/aau-network-security/go-ntp/logging"
	"github.com/mholt/certmagic"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xenolf/lego/providers/dns/cloudflare"
	"os/user"
)

var (
	DuplicateEventErr   = errors.New("Event with that tag already exists")
	UnknownEventErr     = errors.New("Unable to find event by that tag")
	MissingTokenErr     = errors.New("No security token provided")
	InvalidArgumentsErr = errors.New("Invalid arguments provided")
	UnknownTeamErr      = errors.New("Unable to find team by that id")

	version string
)

type MissingConfigErr struct {
	Option string
}

func (m *MissingConfigErr) Error() string {
	return fmt.Sprintf("%s cannot be empty", m.Option)
}

type Config struct {
	Host struct {
		Http string `yaml:"http,omitempty"`
		Grpc string `yaml:"grpc,omitempty"`
	} `yaml:"host,omitempty"`
	Port struct {
		Secure   uint `yaml:"secure,omitempty"`
		InSecure uint `yaml:"insecure,omitempty"`
	}
	UsersFile          string                           `yaml:"users-file,omitempty"`
	ExercisesFile      string                           `yaml:"exercises-file,omitempty"`
	FrontendsFile      string                           `yaml:"frontends-file,omitempty"`
	OvaDir             string                           `yaml:"ova-directory,omitempty"`
	LogDir             string                           `yaml:"log-directory,omitempty"`
	EventsDir          string                           `yaml:"events-directory,omitempty"`
	DockerRepositories []dockerclient.AuthConfiguration `yaml:"docker-repositories,omitempty"`
	SigningKey         string                           `yaml:"sign-key,omitempty"`
	TLS                struct {
		Enabled   bool   `yaml:"enabled"`
		Directory string `yaml:"directory"`
		ACME      struct {
			Email       string `yaml:"email"`
			ApiKey      string `yaml:"api-key"`
			Development bool   `yaml:"development"`
		} `yaml:"acme"`
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

func New(conf *Config) (*daemon, error) {
	if conf.SigningKey == "" {
		return nil, &MissingConfigErr{"Management signing key"}
	}

	if conf.Host.Http == "" {
		conf.Host.Http = "localhost"
	}

	if conf.Host.Grpc == "" {
		conf.Host.Grpc = "localhost"
	}

	if conf.Port.InSecure == 0 {
		conf.Port.InSecure = 80
	}

	if conf.Port.Secure == 0 {
		conf.Port.Secure = 443
	}

	if conf.OvaDir == "" {
		dir, _ := os.Getwd()
		conf.OvaDir = filepath.Join(dir, "vbox")
	}

	if conf.LogDir == "" {
		dir, _ := os.Getwd()
		conf.LogDir = filepath.Join(dir, "logs")
	}

	if conf.UsersFile == "" {
		conf.UsersFile = "users.yml"
	}

	if conf.ExercisesFile == "" {
		conf.ExercisesFile = "exercises.yml"
	}

	if conf.FrontendsFile == "" {
		conf.FrontendsFile = "frontends.yml"
	}

	if conf.EventsDir == "" {
		conf.EventsDir = "events"
	}

	var domains []string
	if conf.TLS.Enabled {
		if conf.TLS.Directory == "" {
			usr, err := user.Current()
			if err != nil {
				return nil, errors.New("Invalid user")
			}
			conf.TLS.Directory = filepath.Join(usr.HomeDir, ".local", "share", "certmagic")
		}

		if err := os.Setenv("CLOUDFLARE_EMAIL", conf.TLS.ACME.Email); err != nil {
			return nil, err
		}

		if err := os.Setenv("CLOUDFLARE_API_KEY", conf.TLS.ACME.ApiKey); err != nil {
			return nil, err
		}

		provider, err := cloudflare.NewDNSProvider()
		if err != nil {
			return nil, err
		}

		domains = []string{
			conf.Host.Grpc,
			fmt.Sprintf("*.%s", conf.Host.Http),
		}

		certmagic.HTTPPort = int(conf.Port.InSecure)
		certmagic.HTTPSPort = int(conf.Port.Secure)
		certmagic.Agreed = true
		certmagic.Email = conf.TLS.ACME.Email
		certmagic.CA = certmagic.LetsEncryptProductionCA
		certmagic.DefaultStorage = &certmagic.FileStorage{
			Path: conf.TLS.Directory,
		}
		if conf.TLS.ACME.Development {
			certmagic.CA = certmagic.LetsEncryptStagingCA
		}
		certmagic.DNSProvider = provider
		_, err = certmagic.Manage(domains)
		if err != nil {
			return nil, err
		}
	}

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
	go func() {
		if conf.TLS.Enabled {
			if err := certmagic.HTTPS(domains, eventPool); err != nil {
				fmt.Println("Serving error", err)
			}
			return
		}

		if err := http.ListenAndServe(fmt.Sprintf(":%d", conf.Port.InSecure), eventPool); err != nil {
			fmt.Println("Serving error", err)
		}
	}()

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

	efh, err := store.NewEventFileHub(conf.EventsDir)
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
		ehost:     event.NewHost(vlib, ef, efh),
		logPool:   logPool,
		closers:   []io.Closer{logPool, eventPool},
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

type contextStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextStream) Context() context.Context {
	return s.ctx
}

func withAuditLogger(ctx context.Context, logger *zerolog.Logger) context.Context {
	if logger == nil {
		return ctx
	}

	u, ok := ctx.Value(us{}).(store.User)
	if !ok {
		return logger.WithContext(ctx)
	}

	ls := logger.With().
		Str("user", u.Username).
		Bool("is-super-user", u.SuperUser).
		Logger()
	logger = &ls

	return logger.WithContext(ctx)
}

func (d *daemon) GetServer(opts ...grpc.ServerOption) *grpc.Server {
	nonAuth := []string{"LoginUser", "SignupUser"}
	var logger *zerolog.Logger
	if d.logPool != nil {
		logger, _ = d.logPool.GetLogger("audit")
		l := *logger
		l = l.With().Timestamp().Logger()
		logger = &l
	}

	streamInterceptor := func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, authErr := d.auth.AuthenticateContext(stream.Context())
		ctx = withAuditLogger(ctx, logger)
		stream = &contextStream{stream, ctx}

		header := metadata.Pairs("daemon-version", version)
		stream.SendHeader(header)

		for _, endpoint := range nonAuth {
			if strings.HasSuffix(info.FullMethod, endpoint) {
				return handler(srv, stream)
			}
		}

		if authErr != nil {
			return authErr
		}

		return handler(srv, stream)
	}

	unaryInterceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, authErr := d.auth.AuthenticateContext(ctx)
		ctx = withAuditLogger(ctx, logger)

		header := metadata.Pairs("daemon-version", version)
		grpc.SendHeader(ctx, header)

		for _, endpoint := range nonAuth {
			if strings.HasSuffix(info.FullMethod, endpoint) {
				return handler(ctx, req)
			}
		}

		if authErr != nil {
			return nil, authErr
		}

		return handler(ctx, req)
	}

	opts = append([]grpc.ServerOption{
		grpc.StreamInterceptor(streamInterceptor),
		grpc.UnaryInterceptor(unaryInterceptor),
	}, opts...)
	return grpc.NewServer(opts...)
}

func (d *daemon) LoginUser(ctx context.Context, req *pb.LoginUserRequest) (*pb.LoginUserResponse, error) {
	log.Ctx(ctx).
		Info().
		Str("username", req.Username).
		Msg("login user")

	token, err := d.auth.TokenForUser(req.Username, req.Password)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	return &pb.LoginUserResponse{Token: token}, nil
}

func (d *daemon) SignupUser(ctx context.Context, req *pb.SignupUserRequest) (*pb.LoginUserResponse, error) {
	log.Ctx(ctx).
		Info().
		Str("username", req.Username).
		Msg("signup user")

	u, err := store.NewUser(req.Username, req.Password)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	k, err := d.users.GetSignupKey(req.Key)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}
	if k.WillBeSuperUser {
		u.SuperUser = true
	}

	if err := d.users.CreateUser(u); err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	if err := d.users.DeleteSignupKey(k); err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	token, err := d.auth.TokenForUser(req.Username, req.Password)
	if err != nil {
		return &pb.LoginUserResponse{Error: err.Error()}, nil
	}

	return &pb.LoginUserResponse{Token: token}, nil
}

func (d *daemon) InviteUser(ctx context.Context, req *pb.InviteUserRequest) (*pb.InviteUserResponse, error) {
	log.Ctx(ctx).Info().Msg("invite user")

	u, _ := ctx.Value(us{}).(store.User)
	if !u.SuperUser {
		return &pb.InviteUserResponse{
			Error: "This action requires super user permissions",
		}, nil
	}

	k := store.NewSignupKey()
	if req.SuperUser {
		k.WillBeSuperUser = true
	}

	if err := d.users.CreateSignupKey(k); err != nil {
		return &pb.InviteUserResponse{
			Error: err.Error(),
		}, nil
	}

	return &pb.InviteUserResponse{
		Key: k.String(),
	}, nil
}

func (d *daemon) createEventFromEventFile(ef store.EventFile) error {
	ev, err := d.ehost.CreateEventFromEventFile(ef)
	if err != nil {
		log.Error().Err(err).Msg("Error creating event from file")
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

	var frontendNames []string
	for _, f := range conf.Lab.Frontends {
		frontendNames = append(frontendNames, f.Image)
	}
	log.Info().
		Str("Name", conf.Name).
		Str("Tag", string(conf.Tag)).
		Int("Available", conf.Available).
		Int("Capacity", conf.Capacity).
		Strs("Frontends", frontendNames).
		Msg("Creating event")

	go ev.Start(context.TODO())

	d.eventPool.AddEvent(ev)

	return nil
}

func (d *daemon) CreateEvent(req *pb.CreateEventRequest, resp pb.Daemon_CreateEventServer) error {
	log.Ctx(resp.Context()).
		Info().
		Str("tag", req.Tag).
		Str("name", req.Name).
		Int32("available", req.Available).
		Int32("capacity", req.Capacity).
		Strs("frontends", req.Frontends).
		Strs("exercises", req.Exercises).
		Msg("create event")
	now := time.Now()

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
		Name:      req.Name,
		Tag:       evtag,
		Available: int(req.Available),
		Capacity:  int(req.Capacity),
		StartedAt: &now,
		Lab: store.Lab{
			Frontends: d.frontends.GetFrontends(req.Frontends...),
			Exercises: tags,
		},
	}

	if err := conf.Validate(); err != nil {
		return err
	}

	_, err := d.eventPool.GetEvent(evtag)
	if err == nil {
		return DuplicateEventErr
	}

	if conf.Available == 0 {
		conf.Available = 5
	}

	if conf.Capacity == 0 {
		conf.Capacity = 10
	}

	return d.createEventFromConfig(conf)
}

func (d *daemon) StopEvent(req *pb.StopEventRequest, resp pb.Daemon_StopEventServer) error {
	log.Ctx(resp.Context()).
		Info().
		Str("tag", req.Tag).
		Msg("stop event")

	evtag, err := store.NewTag(req.Tag)
	if err != nil {
		return err
	}

	ev, err := d.eventPool.GetEvent(evtag)
	if err != nil {
		return err
	}

	if err := d.eventPool.RemoveEvent(evtag); err != nil {
		return err
	}

	ev.Close()
	ev.Finish()
	return nil
}

func (d *daemon) RestartTeamLab(req *pb.RestartTeamLabRequest, resp pb.Daemon_RestartTeamLabServer) error {
	log.Ctx(resp.Context()).
		Info().
		Str("event", req.EventTag).
		Str("lab", req.LabTag).
		Msg("restart lab")

	evtag, err := store.NewTag(req.EventTag)
	if err != nil {
		return err
	}

	ev, err := d.eventPool.GetEvent(evtag)
	if err != nil {
		return err
	}

	lab, err := ev.GetHub().GetLabByTag(req.LabTag)

	if err != nil {
		return err
	}

	if err := lab.Restart(resp.Context()); err != nil {
		return err
	}

	return nil
}

func (d *daemon) ListExercises(ctx context.Context, req *pb.Empty) (*pb.ListExercisesResponse, error) {

	var exercises []*pb.ListExercisesResponse_Exercise

	for _, e := range d.exercises.ListExercises() {
		var tags []string
		for _, t := range e.Tags {
			tags = append(tags, string(t))
		}

		exercises = append(exercises, &pb.ListExercisesResponse_Exercise{
			Name:             e.Name,
			Tags:             tags,
			DockerImageCount: int32(len(e.DockerConfs)),
			VboxImageCount:   int32(len(e.VboxConfs)),
		})
	}

	return &pb.ListExercisesResponse{Exercises: exercises}, nil
}

func (d *daemon) ResetExercise(req *pb.ResetExerciseRequest, stream pb.Daemon_ResetExerciseServer) error {
	log.Ctx(stream.Context()).Info().
		Str("evtag", req.EventTag).
		Str("extag", req.ExerciseTag).
		Msg("reset exercise")

	evtag, err := store.NewTag(req.EventTag)
	if err != nil {
		return err
	}

	ev, err := d.eventPool.GetEvent(evtag)
	if err != nil {
		return err
	}

	if req.Teams != nil {
		for _, reqTeam := range req.Teams {
			lab, ok := ev.GetLabByTeam(reqTeam.Id)
			if !ok {
				stream.Send(&pb.ResetTeamStatus{TeamId: reqTeam.Id, Status: "?"})
				continue
			}

			if err := lab.GetEnvironment().ResetByTag(stream.Context(), req.ExerciseTag); err != nil {
				return err
			}
			stream.Send(&pb.ResetTeamStatus{TeamId: reqTeam.Id, Status: "ok"})
		}

		return nil
	}

	for _, t := range ev.GetTeams() {
		lab, ok := ev.GetLabByTeam(t.Id)
		if !ok {
			stream.Send(&pb.ResetTeamStatus{TeamId: t.Id, Status: "?"})
			continue
		}

		if err := lab.GetEnvironment().ResetByTag(stream.Context(), req.ExerciseTag); err != nil {
			return err
		}
		stream.Send(&pb.ResetTeamStatus{TeamId: t.Id, Status: "ok"})
	}

	return nil
}

func (d *daemon) ListEvents(ctx context.Context, req *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	var events []*pb.ListEventsResponse_Events

	for _, event := range d.eventPool.GetAllEvents() {
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
	ev, err := d.eventPool.GetEvent(evtag)
	if err != nil {
		return nil, err
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

func (d *daemon) Close() error {
	var errs error
	var wg sync.WaitGroup

	for _, c := range d.closers {
		wg.Add(1)
		go func(c io.Closer) {
			if err := c.Close(); err != nil && errs == nil {
				errs = err
			}
			wg.Done()
		}(c)
	}

	wg.Wait()

	if err := docker.DefaultLinkBridge.Close(); err != nil {
		return err
	}

	return errs
}

func (d *daemon) ListFrontends(ctx context.Context, req *pb.Empty) (*pb.ListFrontendsResponse, error) {
	var respList []*pb.ListFrontendsResponse_Frontend

	err := filepath.Walk(d.conf.OvaDir, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".ova" {
			relativePath, err := filepath.Rel(d.conf.OvaDir, path)
			if err != nil {
				return err
			}
			parts := strings.Split(relativePath, ".")
			image := filepath.Join(parts[:len(parts)-1]...)

			ic := d.frontends.GetFrontends(image)[0]
			respList = append(respList, &pb.ListFrontendsResponse_Frontend{
				Image:    image,
				Size:     info.Size(),
				MemoryMB: int64(ic.MemoryMB),
				Cpu:      float32(ic.CPU),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &pb.ListFrontendsResponse{Frontends: respList}, nil
}

func (d *daemon) ResetFrontends(req *pb.ResetFrontendsRequest, stream pb.Daemon_ResetFrontendsServer) error {
	log.Ctx(stream.Context()).Info().
		Int("n-teams", len(req.Teams)).
		Msg("reset frontends")

	evtag, err := store.NewTag(req.EventTag)
	if err != nil {
		return err
	}

	ev, err := d.eventPool.GetEvent(evtag)
	if err != nil {
		return err
	}

	if req.Teams != nil {
		// the requests has a selection of group ids
		for _, reqTeam := range req.Teams {
			lab, ok := ev.GetLabByTeam(reqTeam.Id)
			if !ok {
				stream.Send(&pb.ResetTeamStatus{TeamId: reqTeam.Id, Status: "?"})
				continue
			}

			if err := lab.ResetFrontends(stream.Context()); err != nil {
				return err
			}
			stream.Send(&pb.ResetTeamStatus{TeamId: reqTeam.Id, Status: "ok"})
		}

		return nil
	}

	for _, t := range ev.GetTeams() {
		lab, ok := ev.GetLabByTeam(t.Id)
		if !ok {
			stream.Send(&pb.ResetTeamStatus{TeamId: t.Id, Status: "?"})
			continue
		}

		if err := lab.ResetFrontends(stream.Context()); err != nil {
			return err
		}
		stream.Send(&pb.ResetTeamStatus{TeamId: t.Id, Status: "ok"})
	}

	return nil
}

func (d *daemon) SetFrontendMemory(ctx context.Context, in *pb.SetFrontendMemoryRequest) (*pb.Empty, error) {
	err := d.frontends.SetMemoryMB(in.Image, uint(in.MemoryMB))
	return &pb.Empty{}, err
}

func (d *daemon) SetFrontendCpu(ctx context.Context, in *pb.SetFrontendCpuRequest) (*pb.Empty, error) {
	err := d.frontends.SetCpu(in.Image, float64(in.Cpu))
	return &pb.Empty{}, err
}

func (d *daemon) GetTeamInfo(ctx context.Context, in *pb.GetTeamInfoRequest) (*pb.GetTeamInfoResponse, error) {
	t, err := store.NewTag(in.EventTag)
	if err != nil {
		return nil, err
	}
	ev, err := d.eventPool.GetEvent(t)
	if err != nil {
		return nil, err
	}
	lab, ok := ev.GetLabByTeam(in.TeamId)
	if !ok {
		return nil, UnknownTeamErr
	}

	var instances []*pb.GetTeamInfoResponse_Instance
	for _, i := range lab.InstanceInfo() {
		instance := &pb.GetTeamInfoResponse_Instance{
			Image: i.Image,
			Type:  i.Type,
			Id:    i.Id,
			State: int32(i.State),
		}
		instances = append(instances, instance)
	}
	return &pb.GetTeamInfoResponse{Instances: instances}, nil

}

func (d *daemon) MonitorHost(req *pb.Empty, stream pb.Daemon_MonitorHostServer) error {
	for {
		var cpuErr string
		var cpuPercent float32
		cpus, err := cpu.Percent(time.Second, false)
		if err != nil {
			cpuErr = err.Error()
		}
		if len(cpus) == 1 {
			cpuPercent = float32(cpus[0])
		}

		var memErr string
		v, err := mem.VirtualMemory()
		if err != nil {
			memErr = err.Error()
		}

		// we should send io at some point
		// io, _ := net.IOCounters(true)

		if err := stream.Send(&pb.MonitorHostResponse{
			CPUPercent:      cpuPercent,
			CPUReadError:    cpuErr,
			MemoryPercent:   float32(v.UsedPercent),
			MemoryReadError: memErr,
		}); err != nil {
			return err
		}
	}
}

func (d *daemon) Version(context.Context, *pb.Empty) (*pb.VersionResponse, error) {
	return &pb.VersionResponse{Version: version}, nil
}
