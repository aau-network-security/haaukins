package event

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"net/url"

	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/svcs"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"io"
	"sync"
)

var (
	RdpConfErr      = errors.New("error too few rdp connections")
	StartingCtfdErr = errors.New("error while starting ctfd")
	StartingGuacErr = errors.New("error while starting guac")
	StartingRevErr  = errors.New("error while starting reverse proxy")
	EmptyNameErr    = errors.New("event requires a name")
	EmptyTagErr     = errors.New("event requires a tag")
)

type Host interface {
	CreateEventFromConfig(store.EventConfig) (Event, error)
	CreateEventFromEventFile(store.EventFile) (Event, error)
}

func NewHost(vlib vbox.Library, elib store.ExerciseStore, efh store.EventFileHub) Host {
	return &eventHost{
		efh:  efh,
		vlib: vlib,
		elib: elib,
	}
}

type eventHost struct {
	efh  store.EventFileHub
	vlib vbox.Library
	elib store.ExerciseStore
}

func (eh *eventHost) CreateEventFromEventFile(ef store.EventFile) (Event, error) {
	conf := ef.Read()
	if err := conf.Validate(); err != nil {
		return nil, err
	}

	exer, err := eh.elib.GetExercisesByTags(conf.Lab.Exercises...)
	if err != nil {
		return nil, err
	}

	labConf := lab.Config{
		Exercises: exer,
		Frontends: conf.Lab.Frontends,
	}
	hub, err := lab.NewHub(labConf, eh.vlib, conf.Available, conf.Capacity)
	if err != nil {
		return nil, err
	}

	return NewEvent(ef, hub)
}

func (eh *eventHost) CreateEventFromConfig(conf store.EventConfig) (Event, error) {
	ef, err := eh.efh.CreateEventFile(conf)
	if err != nil {
		return nil, err
	}

	return eh.CreateEventFromEventFile(ef)
}

type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Event interface {
	Start(context.Context) error
	Close()
	Finish()
	AssignLab(store.Team) error
	Connect(*mux.Router)

	GetConfig() store.EventConfig
	GetTeams() []store.Team
	GetHub() lab.Hub
	GetLabByTeam(teamId string) (lab.Lab, bool)
}

type event struct {
	ctfd   ctfd.CTFd
	guac   guacamole.Guacamole
	labhub lab.Hub

	labs  map[string]lab.Lab
	store store.EventFile

	guacUserStore *guacamole.GuacUserStore
	dockerHost    docker.Host

	closers []io.Closer
}

func NewEvent(ef store.EventFile, hub lab.Hub) (Event, error) {
	conf := ef.Read()
	ctfdConf := ctfd.Config{
		Name:  conf.Name,
		Flags: hub.Flags(),
		Teams: ef.GetTeams(),
	}

	ctf, err := ctfd.New(ctfdConf)
	if err != nil {
		return nil, err
	}

	guac, err := guacamole.New(guacamole.Config{})
	if err != nil {
		return nil, err
	}

	dockerHost := docker.NewHost()

	ev := &event{
		store:         ef,
		labhub:        hub,
		ctfd:          ctf,
		guac:          guac,
		labs:          map[string]lab.Lab{},
		guacUserStore: guacamole.NewGuacUserStore(),
		dockerHost:    dockerHost,
	}
	ev.closers = append(ev.closers, ctf, guac, hub)

	return ev, nil
}

func (ev *event) Start(ctx context.Context) error {
	if err := ev.ctfd.Start(); err != nil {
		log.
			Error().
			Err(err).
			Msg("error starting ctfd")

		return StartingCtfdErr
	}

	if err := ev.guac.Start(ctx); err != nil {
		log.
			Error().
			Err(err).
			Msg("error starting guac")

		return StartingGuacErr
	}

	return nil
}

func (ev *event) Close() {
	var wg sync.WaitGroup

	for _, closer := range ev.closers {
		wg.Add(1)
		go func(c io.Closer) {
			if err := c.Close(); err != nil {
				log.Warn().Msgf("error while closing event '%s': %s", ev.GetConfig().Name, err)
			}
			wg.Done()
		}(closer)

	}
	wg.Wait()
}

func (ev *event) Finish() {
	now := time.Now()
	ev.store.Finish(now)
}

func (ev *event) AssignLab(t store.Team) error {
	lab, err := ev.labhub.Get()
	if err != nil {
		return err
	}

	rdpPorts := lab.RdpConnPorts()
	if n := len(rdpPorts); n == 0 {
		log.
			Debug().
			Int("amount", n).
			Msg("Too few RDP connections")

		return RdpConfErr
	}
	u := guacamole.GuacUser{
		Username: t.Id,
		Password: t.HashedPassword,
	}

	if err := ev.guac.CreateUser(u.Username, u.Password); err != nil {
		return err
	}

	ev.guacUserStore.CreateUserForTeam(t.Id, u)

	hostIp, err := ev.dockerHost.GetDockerHostIP()
	if err != nil {
		return err
	}

	for i, port := range rdpPorts {
		num := i + 1
		name := fmt.Sprintf("%s-client%d", t.Id, num)

		log.Debug().Str("team", t.Name).Uint("port", port).Msg("Creating RDP Connection for group")
		if err := ev.guac.CreateRDPConn(guacamole.CreateRDPConnOpts{
			Host:     hostIp,
			Port:     port,
			Name:     name,
			GuacUser: u.Username,
			Username: &u.Username,
			Password: &u.Password,
		}); err != nil {
			return err
		}
	}

	ev.labs[t.Id] = lab

	return nil
}

func (ev *event) Connect(r *mux.Router) {
	prehooks := []func() error{
		func() error {
			if ev.labhub.Available() == 0 {
				return lab.CouldNotFindLabErr
			}
			return nil
		},
	}

	posthooks := []func(t store.Team) error{
		ev.AssignLab,
	}

	loginFunc := func(u string, p string) (string, error) {
		content, err := ev.guac.RawLogin(u, p)
		if err != nil {
			return "", err
		}
		return url.QueryEscape(string(content)), nil
	}

	defaultTasks := []store.Task{}
	for _, f := range ev.labhub.Flags() {
		defaultTasks = append(defaultTasks, store.Task{FlagTag: f.Tag})
	}

	interceptors := svcs.Interceptors{
		ctfd.NewRegisterInterception(ev.store, prehooks, posthooks, defaultTasks...),
		ctfd.NewCheckFlagInterceptor(ev.store, ev.ctfd.ChalMap()),
		ctfd.NewLoginInterceptor(ev.store),
		guacamole.NewGuacTokenLoginEndpoint(ev.guacUserStore, ev.store, loginFunc),
	}
	r.Use(interceptors.Intercept)
	r.HandleFunc("/guacamole{rest:.*}", handler(ev.guac.ProxyHandler()))
	r.HandleFunc("/{rest:.*}", handler(ev.ctfd.ProxyHandler()))
}

func handler(h http.Handler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = mux.Vars(r)["rest"]
		h.ServeHTTP(w, r)
	}
}

func (ev *event) GetHub() lab.Hub {
	return ev.labhub
}

func (ev *event) GetConfig() store.EventConfig {
	return ev.store.Read()
}

func (ev *event) GetTeams() []store.Team {
	return ev.store.GetTeams()
}

func (ev *event) GetLabByTeam(teamId string) (lab.Lab, bool) {
	lab, ok := ev.labs[teamId]
	return lab, ok
}
