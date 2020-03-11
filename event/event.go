// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package event

import (
	"context"
	"errors"
	"fmt"
	"github.com/aau-network-security/haaukins"
	"net/http"
	"time"

	"io"
	"sync"

	"github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/store"
	//"github.com/aau-network-security/haaukins/svcs/ctfd"
	"github.com/aau-network-security/haaukins/svcs/amigo"
	"github.com/aau-network-security/haaukins/svcs/guacamole"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/rs/zerolog/log"
)

var (
	RdpConfErr      = errors.New("error too few rdp connections")
	StartingCtfdErr = errors.New("error while starting ctfd")
	StartingGuacErr = errors.New("error while starting guac")
	StartingRevErr  = errors.New("error while starting reverse proxy")
	EmptyNameErr    = errors.New("event requires a name")
	EmptyTagErr     = errors.New("event requires a tag")

	ErrMaxLabs         = errors.New("maximum amount of allowed labs has been reached")
	ErrNoAvailableLabs = errors.New("no labs available in the queue")
)

type Host interface {
	CreateEventFromConfig(context.Context, store.EventConfig) (Event, error)
	CreateEventFromEventFile(context.Context, store.EventFile) (Event, error)
	UpdateEventHostExercisesFile(store.ExerciseStore) error
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

func (eh *eventHost) UpdateEventHostExercisesFile(es store.ExerciseStore) error {
	if len(es.ListExercises()) == 0 {
		return errors.New("Provided exercisestore is empty, be careful next time ! ")
	}
	eh.elib = es
	return nil
}

func (eh *eventHost) CreateEventFromEventFile(ctx context.Context, ef store.EventFile) (Event, error) {
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

	lh := lab.LabHost{
		Vlib: eh.vlib,
		Conf: labConf,
	}
	hub, err := lab.NewHub(ctx, &lh, conf.Available, conf.Capacity)
	if err != nil {
		return nil, err
	}

	return NewEvent(eh.ctx, ef, hub, labConf.Flags())
}

func (eh *eventHost) CreateEventFromConfig(ctx context.Context, conf store.EventConfig) (Event, error) {
	ef, err := eh.efh.CreateEventFile(conf)
	if err != nil {
		return nil, err
	}

	return eh.CreateEventFromEventFile(ctx, ef)
}

type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Event interface {
	Start(context.Context) error
	Close() error
	Finish()
	AssignLab(*haaukins.Team, lab.Lab) error
	Handler() http.Handler

	GetConfig() store.EventConfig
	//GetTeams() []haaukins.Team
	GetHub() lab.Hub
	GetLabByTeam(teamId string) (lab.Lab, bool)
}

type event struct {
	amigo   *amigo.Amigo
	guac   guacamole.Guacamole
	labhub lab.Hub

	labs          map[string]lab.Lab
	store         store.EventFile
	keyLoggerPool guacamole.KeyLoggerPool

	guacUserStore *guacamole.GuacUserStore
	dockerHost    docker.Host

	closers []io.Closer
}


func NewEvent(ctx context.Context, ef store.EventFile, hub lab.Hub, flags []store.FlagConfig) (Event, error) {
	//conf := ef.Read()
	//ctfdConf := ctfd.Config{
	//	Name:  conf.Name,
	//	Flags: flags,
	//	Teams: ef.GetTeams(),
	//}
	//teams :=ef.GetTeams()

	//teams := ef.GetTeams()
	//team := haaukins.NewTeam("test1@test.dk", "TestingTeamOne", "123456")
	//team2 := haaukins.NewTeam("test@test.dk", "TestingTeam2", "123456")
	chals := []haaukins.Challenge{}
	for _, f := range flags {
		//chalTag := string(f.Tag)
		t, _ := haaukins.NewTag(string(f.Tag))
		chals = append(chals, haaukins.Challenge{Tag:t,Name:f.Name})
	}
	//if teams !=nil {
	//	for _, t := range teams {
	//		//hTeam :=haaukins.NewTeam(t.Name(),t.Email(),"123")
	//		for _, ch := range chals {
	//			 t.AddChallenge(ch)
	//		}
	//	}
	//}

	//ts := store.NewTeamStore(store.WithTeams(ef.GetTeams()),store.WithPostTeamHook)

	//chals := []haaukins.Challenge{{, "Heartbleed"},{"AAA", "Test"}}
	//chals = chals,append(chals,amigo.)
	//amigoConf := amigo.NewAmigo()
	//
	//ctf, err := ctfd.New(ctx, ctfdConf)
	//if err != nil {
	//	return nil, err
	//}


	guac, err := guacamole.New(ctx, guacamole.Config{})
	if err != nil {
		return nil, err
	}

	dockerHost := docker.NewHost()
	amigoOpt := amigo.WithEventName(ef.Read().Name)
	keyLoggerPool, err := guacamole.NewKeyLoggerPool(ef.ArchiveDir())
	if err != nil {
		return nil, err
	}

	ev := &event{
		store:         ef,
		labhub:        hub,
		amigo:         amigo.NewAmigo(store.NewTeamStore(),chals,"testing purposes",amigoOpt),
		guac:          guac,
		labs:          map[string]lab.Lab{},
		guacUserStore: guacamole.NewGuacUserStore(),
		closers:       []io.Closer{ guac, hub, keyLoggerPool},
		dockerHost:    dockerHost,
		keyLoggerPool: keyLoggerPool,
	}

	return ev, nil
}

func (ev *event) Start(ctx context.Context) error {
	//if err := ev.ctfd.Start(ctx); err != nil {
	//	log.
	//		Error().
	//		Err(err).
	//		Msg("error starting ctfd")
	//
	//	return StartingCtfdErr
	//}

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

		//ev.amigo.teamStore.SaveTeam(team)
		ev.store.SaveTeam(team)
		//ev.store.CreateTeam(team)



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
			defer wg.Done()
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

func (ev *event) AssignLab(t *haaukins.Team, lab lab.Lab) error {
	rdpPorts := lab.RdpConnPorts()
	if n := len(rdpPorts); n == 0 {
		log.
			Debug().
			Int("amount", n).
			Msg("Too few RDP connections")

		return RdpConfErr
	}
	u := guacamole.GuacUser{
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

	ev.labs[t.ID()] = lab
	chals := lab.Environment().Challenges()

	for _, chal := range chals {
		tag, _:= haaukins.NewTag(string(chal.FlagTag))
		t.AddChallenge(haaukins.Challenge{tag,chal.OwnerID})
		log.Info().Str("chal-tag", string(chal.FlagTag)).
			Str("chal-val", chal.FlagValue).
			Msgf("Flag is created for team %s [assignlab function] ", t.Name())
	}

	return nil
}

func (ev *event) Handler() http.Handler {
	reghook := func(t *haaukins.Team) error {
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

	guacHandler := ev.guac.ProxyHandler(ev.guacUserStore, ev.keyLoggerPool,ev.amigo)(ev.store)

	//m := http.NewServeMux()
	//m.Handle("/guaclogin", guacHandler)
	//m.Handle("/guacamole", guacHandler)
	//m.Handle("/guacamole/", guacHandler)
	//m.Handle("/", )

	return  ev.amigo.Handler(reghook,guacHandler)
}

func (ev *event) GetHub() lab.Hub {
	return ev.labhub
}

func (ev *event) GetConfig() store.EventConfig {
	return ev.store.Read()
}

//func (ev *event) GetTeams() []haaukins.Team {
//	return ev.store.GetTeams()
//}

func (ev *event) GetLabByTeam(teamId string) (lab.Lab, bool) {
	lab, ok := ev.labs[teamId]
	return lab, ok
}
