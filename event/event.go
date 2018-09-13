package event

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
	"github.com/aau-network-security/go-ntp/svcs/revproxy"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var (
	RdpConfErr      = errors.New("error too few rdp connections")
	StartingCtfdErr = errors.New("error while starting ctfd")
	StartingGuacErr = errors.New("error while starting guac")
	StartingRevErr  = errors.New("error while starting reverse proxy")
	EmptyNameErr    = errors.New("event requires a name")
	EmptyTagErr     = errors.New("event requires a tag")

	getDockerHostIp = docker.GetDockerHostIP
)

type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Group struct {
	Name string
}

type Event interface {
	Start(context.Context) error
	Close()
	Register(Group) (*Auth, error)
}

type event struct {
	ctfd   ctfd.CTFd
	proxy  revproxy.Proxy
	guac   guacamole.Guacamole
	cbSrv  *callbackServer
	labhub lab.Hub
}

func rand() string {
	return strings.Replace(fmt.Sprintf("%v", uuid.New()), "-", "", -1)
}

func NewFromFile(path string, opts ...EventOpt) (Event, error) {
	conf, err := loadConfig(path)
	if err != nil {
		return nil, err
	}

	return New(*conf, opts...)
}

func New(conf Config, opts ...EventOpt) (Event, error) {
	if conf.Name == "" {
		return nil, EmptyNameErr
	}

	if conf.Tag == "" {
		return nil, EmptyTagErr
	}

	conf.CTFd.Name = conf.Name

	ev := &event{}

	for _, opt := range opts {
		opt(ev)
	}

	cb := &callbackServer{event: ev}
	if err := cb.Run(); err != nil {
		return nil, err
	}
	ev.cbSrv = cb

	if ev.labhub == nil {
		labHub, err := lab.NewHub(conf.Lab)
		if err != nil {
			return nil, err
		}

		ev.labhub = labHub
	}

	if ev.ctfd == nil {
		// TODO: this is not implemented with dynamic flags in mind; dynamic flag string can simply not be specified in the initial config
		conf.CTFd.Flags = ev.labhub.Flags()
		conf.CTFd.CallbackHost = ev.cbSrv.host
		conf.CTFd.CallbackPort = ev.cbSrv.port

		ctf, err := ctfd.New(conf.CTFd)
		if err != nil {
			return nil, err
		}

		ev.ctfd = ctf
	}

	if ev.guac == nil {
		guac, err := guacamole.New(conf.Guac)
		if err != nil {
			return nil, err
		}

		ev.guac = guac
	}

	proxy, err := revproxy.New(conf.Proxy, ev.ctfd, ev.guac)
	if err != nil {
		return nil, err
	}

	ev.proxy = proxy

	return ev, nil
}

type EventOpt func(*event)

func WithCTFd(ctf ctfd.CTFd) EventOpt {
	return func(e *event) {
		e.ctfd = ctf
	}
}

func WithGuacamole(guac guacamole.Guacamole) EventOpt {
	return func(e *event) {
		e.guac = guac
	}
}

func WithLabHub(hub lab.Hub) EventOpt {
	return func(e *event) {
		e.labhub = hub
	}
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

	if err := ev.proxy.Start(ctx); err != nil {
		log.
			Error().
			Err(err).
			Msg("error starting reverse proxy")

		return StartingRevErr
	}

	return nil
}

func (ev *event) Close() {
	if ev.proxy != nil {
		ev.proxy.Close()
	}
	if ev.guac != nil {
		ev.guac.Close()
	}
	if ev.ctfd != nil {
		ev.ctfd.Close()
	}
	if ev.labhub != nil {
		ev.labhub.Close()
	}

	if ev.cbSrv != nil {
		ev.cbSrv.Close()
	}
}

func (ev *event) Register(group Group) (*Auth, error) {
	lab, err := ev.labhub.Get()
	if err != nil {
		return nil, err
	}

	rdpPorts := lab.RdpConnPorts()
	if n := len(rdpPorts); n == 0 {
		log.
			Debug().
			Int("amount", n).
			Msg("Too few RDP connections")

		return nil, RdpConfErr
	}

	auth := Auth{
		Username: group.Name,
		Password: rand(),
	}

	if err := ev.guac.CreateUser(auth.Username, auth.Password); err != nil {
		return nil, err
	}

	hostIp, err := getDockerHostIp()
	if err != nil {
		return nil, err
	}

	for i, port := range rdpPorts {
		num := i + 1
		name := fmt.Sprintf("%s-client%d", group.Name, num)

		log.Debug().Str("group", group.Name).Uint("port", port).Msg("Creating RDP Connection for group")
		if err := ev.guac.CreateRDPConn(guacamole.CreateRDPConnOpts{
			Host:     hostIp,
			Port:     port,
			Name:     name,
			GuacUser: auth.Username,
			Username: &auth.Username,
			Password: &auth.Password,
		}); err != nil {
			return nil, err
		}

	}

	return &auth, nil
}
