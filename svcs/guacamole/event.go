// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package guacamole

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"net/http"
	"path/filepath"
	"time"

	pbc "github.com/aau-network-security/haaukins/store/proto"

	"io"
	"sync"

	"github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/amigo"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/rs/zerolog/log"
)

const (
	displayTimeFormat = "2006-01-02 15:04:05"
	Running           = State(0)
	Suspended         = State(1)
	Booked            = State(2)
	Closed            = State(3)
	Error             = State(4)
)

type State int

var (
	RdpConfErr      = errors.New("error too few rdp connections")
	StartingGuacErr = errors.New("error while starting guac")
	//EmptyNameErr    = errors.New("event requires a name")
	//EmptyTagErr     = errors.New("event requires a tag")

	ErrMaxLabs         = errors.New("maximum amount of allowed labs has been reached")
	ErrNoAvailableLabs = errors.New("no labs available in the queue")
)

type Host interface {
	UpdateEventHostExercisesFile(store.ExerciseStore) error
	CreateEventFromEventDB(context.Context, store.EventConfig) (Event, error)
	CreateEventFromConfig(context.Context, store.EventConfig) (Event, error)
}

func NewHost(vlib vbox.Library, elib store.ExerciseStore, eDir string, dbc pbc.StoreClient) Host {
	return &eventHost{
		ctx:  context.Background(),
		dbc:  dbc,
		vlib: vlib,
		elib: elib,
		dir:  eDir,
	}
}

type eventHost struct {
	ctx  context.Context
	dbc  pbc.StoreClient
	vlib vbox.Library
	elib store.ExerciseStore
	dir  string
}

//Create the event configuration for the event got from the DB
func (eh *eventHost) CreateEventFromEventDB(ctx context.Context, conf store.EventConfig) (Event, error) {
	exer, err := eh.elib.GetExercisesByTags(conf.Lab.Exercises...)
	if err != nil {
		return nil, err
	}

	labConf := lab.Config{
		Exercises: exer,
		Frontends: conf.Lab.Frontends,
	}

	lh := lab.LabHost{
		Vlib: eh.vlib,
		Conf: labConf,
	}
	hub, err := lab.NewHub(ctx, &lh, conf.Available, conf.Capacity)
	if err != nil {
		return nil, err
	}

	es, err := store.NewEventStore(conf, eh.dir, eh.dbc)
	if err != nil {
		return nil, err
	}

	return NewEvent(eh.ctx, es, hub, labConf.Flags())
}

//Save the event in the DB and create the event configuration
func (eh *eventHost) CreateEventFromConfig(ctx context.Context, conf store.EventConfig) (Event, error) {
	var exercises []string
	for _, e := range conf.Lab.Exercises {
		exercises = append(exercises, string(e))
	}
	_, err := eh.dbc.AddEvent(ctx, &pbc.AddEventRequest{
		Name:               conf.Name,
		Tag:                string(conf.Tag),
		Frontends:          conf.Lab.Frontends[0].Image,
		Exercises:          strings.Join(exercises, ","),
		Available:          int32(conf.Available),
		Capacity:           int32(conf.Capacity),
		Status:             int32(conf.Status),
		StartTime:          conf.StartedAt.Format(displayTimeFormat),
		ExpectedFinishTime: conf.FinishExpected.Format(displayTimeFormat),
		CreatedBy:          conf.CreatedBy,
	})

	if err != nil {
		return nil, err
	}

	return eh.CreateEventFromEventDB(ctx, conf)
}

func (eh *eventHost) UpdateEventHostExercisesFile(es store.ExerciseStore) error {
	if len(es.ListExercises()) == 0 {
		return errors.New("Provided exercisestore is empty, be careful next time ! ")
	}
	eh.elib = es
	return nil
}

type Event interface {
	Start(context.Context) error
	Close() error
	Suspend(context.Context) error
	Resume(context.Context) error

	Finish(string)
	AssignLab(*store.Team, lab.Lab) error
	Handler() http.Handler

	SetStatus(int32)
	GetStatus() int32

	GetConfig() store.EventConfig
	GetTeams() []*store.Team
	GetHub() lab.Hub
	GetLabByTeam(teamId string) (lab.Lab, bool)
}

type event struct {
	amigo  *amigo.Amigo
	guac   Guacamole
	labhub lab.Hub

	labs          map[string]lab.Lab
	store         store.Event
	keyLoggerPool KeyLoggerPool

	guacUserStore *GuacUserStore
	dockerHost    docker.Host

	closers []io.Closer
}

func NewEvent(ctx context.Context, e store.Event, hub lab.Hub, flags []store.FlagConfig) (Event, error) {

	guac, err := New(ctx, Config{})
	if err != nil {
		return nil, err
	}

	// todo: could be removed
	dirname, err := store.GetDirNameForEvent(e.Dir, e.Tag, e.StartedAt)
	if err != nil {
		return nil, err
	}

	dockerHost := docker.NewHost()
	amigoOpt := amigo.WithEventName(e.Name)
	keyLoggerPool, err := NewKeyLoggerPool(filepath.Join(e.Dir, dirname))
	if err != nil {
		return nil, err
	}

	ev := &event{
		store:         e,
		labhub:        hub,
		amigo:         amigo.NewAmigo(e, flags, amigoOpt),
		guac:          guac,
		labs:          map[string]lab.Lab{},
		guacUserStore: NewGuacUserStore(),
		closers:       []io.Closer{guac, hub, keyLoggerPool},
		dockerHost:    dockerHost,
		keyLoggerPool: keyLoggerPool,
	}

	return ev, nil
}

// SetStatus sets status of event in cache
func (ev *event) SetStatus(state int32) {
	ev.store.Status = state
}

func (ev *event) GetStatus() int32 {
	return ev.store.Status
}

func (ev *event) Start(ctx context.Context) error {

	if err := ev.guac.Start(ctx); err != nil {
		log.
			Error().
			Err(err).
			Msg("error starting guac")

		return StartingGuacErr
	}

	for _, team := range ev.store.GetTeams() {
		lab, ok := <-ev.labhub.Queue()
		if !ok {
			return ErrMaxLabs
		}

		if err := ev.AssignLab(team, lab); err != nil {
			fmt.Println("Issue assigning lab: ", err)
			return err
		}

	}

	return nil
}

//Suspend function suspends event by using from event hub.
func (ev *event) Suspend(ctx context.Context) error {
	var teamLabSuspendError error
	if err := ev.labhub.Suspend(ctx); err != nil {
		return err
	}

	if err := ev.store.SetStatus(string(ev.store.Tag), int32(Suspended)); err != nil {
		return err
	}
	return teamLabSuspendError
}

//Resume function resumes event by using event hub
func (ev *event) Resume(ctx context.Context) error {
	var teamLabResumeError error
	if err := ev.labhub.Resume(ctx); err != nil {
		return err
	}

	// sets status of the event on haaukins store
	if err := ev.store.SetStatus(string(ev.store.Tag), int32(Running)); err != nil {
		return err
	}

	return teamLabResumeError
}

func (ev *event) Close() error {
	var wg sync.WaitGroup

	for _, closer := range ev.closers {
		wg.Add(1)
		go func(c io.Closer) {
			if err := c.Close(); err != nil {
				log.Warn().Msgf("error while closing event '%s': %s", ev.GetConfig().Name, err)
			}
			defer wg.Done()
		}(closer)
	}
	wg.Wait()

	return nil
}

func (ev *event) Finish(newTag string) {
	now := time.Now()
	err := ev.store.Finish(newTag, now)
	if err != nil {
		log.Warn().Msgf("error while archiving event: %s", err)
	}
}

func (ev *event) AssignLab(t *store.Team, lab lab.Lab) error {
	enableWallPaper := true
	rdpPorts := lab.RdpConnPorts()
	if n := len(rdpPorts); n == 0 {
		log.
			Debug().
			Int("amount", n).
			Msg("Too few RDP connections")

		return RdpConfErr
	}
	u := GuacUser{
		Username: t.Name(),
		Password: t.GetHashedPassword(),
	}

	if err := ev.guac.CreateUser(u.Username, u.Password); err != nil {
		log.
			Debug().
			Str("err", err.Error()).
			Msg("Unable to create guacamole user")
		return err
	}

	ev.guacUserStore.CreateUserForTeam(t.ID(), u)

	hostIp, err := ev.dockerHost.GetDockerHostIP()
	if err != nil {
		return err
	}

	for i, port := range rdpPorts {
		num := i + 1
		name := fmt.Sprintf("%s-client%d", t.ID(), num)

		log.Debug().Str("team", t.Name()).Uint("port", port).Msg("Creating RDP Connection for group")
		if err := ev.guac.CreateRDPConn(CreateRDPConnOpts{
			Host:            hostIp,
			Port:            port,
			Name:            name,
			GuacUser:        u.Username,
			Username:        &u.Username,
			Password:        &u.Password,
			EnableWallPaper: &enableWallPaper,
		}); err != nil {
			return err
		}
	}

	ev.labs[t.ID()] = lab
	chals := lab.Environment().Challenges()

	for _, chal := range chals {
		tag, _ := store.NewTag(string(chal.Tag))
		f, _ := t.AddChallenge(store.Challenge{
			Tag:   tag,
			Name:  chal.Name,
			Value: chal.Value,
		})
		log.Info().Str("chal-tag", string(tag)).
			Str("chal-val", f.String()).
			Msgf("Flag is created for team %s [assignlab function] ", t.Name())
	}
	return nil
}

func (ev *event) Handler() http.Handler {

	reghook := func(t *store.Team) error {
		select {
		case lab, ok := <-ev.labhub.Queue():
			if !ok {
				return ErrMaxLabs
			}

			if err := ev.AssignLab(t, lab); err != nil {
				return err
			}
		default:
			return ErrNoAvailableLabs
		}

		return nil
	}

	guacHandler := ev.guac.ProxyHandler(ev.guacUserStore, ev.keyLoggerPool, ev.amigo, ev)(ev.store)

	return ev.amigo.Handler(reghook, guacHandler)
}

func (ev *event) GetHub() lab.Hub {
	return ev.labhub
}

func (ev *event) GetConfig() store.EventConfig {
	return ev.store.EventConfig
}

func (ev *event) GetTeams() []*store.Team {
	return ev.store.GetTeams()
}

func (ev *event) GetLabByTeam(teamId string) (lab.Lab, bool) {
	lab, ok := ev.labs[teamId]
	return lab, ok
}
