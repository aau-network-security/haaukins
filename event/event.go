// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package event

import (
	"context"
	"errors"
	"fmt"
	"github.com/aau-network-security/haaukins/lab"
	"net/http"
	"time"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/ctfd"
	"github.com/aau-network-security/haaukins/svcs/guacamole"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/aau-network-security/haaukins/virtual/vbox"
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
		ctx:  context.Background(),
		efh:  efh,
		vlib: vlib,
		elib: elib,
	}
}

type eventHost struct {
	ctx  context.Context
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

	return NewEvent(eh.ctx, ef, hub)
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
	Close() error
	Finish()
	AssignLab(*store.Team) error
	Handler() http.Handler

	GetConfig() store.EventConfig
	GetTeams() []store.Team
	GetHub() lab.Hub
	GetLabByTeam(teamId string) (lab.Lab, bool)

}

type event struct {
	ctfd   ctfd.CTFd
	guac   guacamole.Guacamole
	labhub lab.Hub

	labs          map[string]lab.Lab
	store         store.EventFile
	keyLoggerPool guacamole.KeyLoggerPool

	guacUserStore *guacamole.GuacUserStore
	dockerHost    docker.Host

	closers []io.Closer
}

func NewEvent(ctx context.Context, ef store.EventFile, hub lab.Hub) (Event, error) {
	conf := ef.Read()
	ctfdConf := ctfd.Config{
		Name:  conf.Name,
		Flags: hub.Flags(),
		Teams: ef.GetTeams(),
	}

	ctf, err := ctfd.New(ctx, ctfdConf)
	if err != nil {
		return nil, err
	}

	guac, err := guacamole.New(ctx, guacamole.Config{})
	if err != nil {
		return nil, err
	}

	dockerHost := docker.NewHost()

	keyLoggerPool, err := guacamole.NewKeyLoggerPool(ef.ArchiveDir())
	if err != nil {
		return nil, err
	}

	ev := &event{
		store:         ef,
		labhub:        hub,
		ctfd:          ctf,
		guac:          guac,
		labs:          map[string]lab.Lab{},
		guacUserStore: guacamole.NewGuacUserStore(),
		closers:       []io.Closer{ctf, guac, hub, keyLoggerPool},
		dockerHost:    dockerHost,
		keyLoggerPool: keyLoggerPool,
	}

	return ev, nil
}

func (ev *event) Start(ctx context.Context) error {
	log.Info().Msg("Starting CTFD container from initial point")
	if err := ev.ctfd.Start(ctx); err != nil {
		log.
			Error().
			Err(err).
			Msg("error starting ctfd")

		return StartingCtfdErr
	}
	log.Info().Msg("Starting GUAC container from initial point")

	if err := ev.guac.Start(ctx); err != nil {
		log.
			Error().
			Err(err).
			Msg("error starting guac")

		return StartingGuacErr
	}


	if len(ev.store.GetTeams()) ==0 {
		log.Warn().Msg("Teams not found, so assigning lab to team function is skipping....")
	}else {
		log.Warn().Str("Team 0 ",ev.store.GetTeams()[0].Name).Msg("0 indexed team....")
	}
	for _, team := range ev.store.GetTeams() {
		if err := ev.AssignLab(&team);
		err != nil {
			//log.Error().Msg("Error while issuing error to team.. Check out assignlab function... ")
			return err
		}

		ev.store.SaveTeam(team)

	}

	return nil
}

func (ev *event) Close() error {
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

	return nil
}

func (ev *event) Finish() {
	now := time.Now()
	ev.store.Finish(now)

	if err := ev.store.Archive(); err != nil {
		log.Warn().Msgf("error while archiving event: %s", err)
	}
}

func (ev *event) AssignLab(t *store.Team) error {
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
		log.
			Error().
			Msg("Error while creating guac user ... ")
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

	fmt.Printf("Instance Info:  %+v\n", ev.labs[t.Id].InstanceInfo())
	chals := lab.GetEnvironment().Challenges()
	for _, chal := range chals {
		t.AddChallenge(chal)
	}

	return nil
}

func (ev *event) Handler() http.Handler {
	reghook := func(t *store.Team) error {
		if ev.labhub.Available() == 0 {
			return lab.CouldNotFindLabErr
		}

		if err := ev.AssignLab(t); err != nil {
			return err
		}

		return nil
	}

	guacHandler := ev.guac.ProxyHandler(ev.guacUserStore, ev.keyLoggerPool)(ev.store)

	m := http.NewServeMux()
	m.Handle("/guaclogin", guacHandler)
	m.Handle("/guacamole", guacHandler)
	m.Handle("/guacamole/", guacHandler)
	m.Handle("/", ev.ctfd.ProxyHandler(reghook)(ev.store))

	return m
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
