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

	ctfdNew         = ctfd.New
	guacNew         = guacamole.New
	proxyNew        = revproxy.New
	labNewHub       = lab.NewHub
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
	labhub lab.Hub
}

func rand() string {
	return strings.Replace(fmt.Sprintf("%v", uuid.New()), "-", "", -1)
}

func New(confPath string) (Event, error) {
	conf, err := loadConfig(confPath)
	if err != nil {
		return nil, err
	}

	labHub, err := labNewHub(conf.Lab)
	if err != nil {
		return nil, err
	}

	// TODO: this is not implemented with dynamic flags in mind; dynamic flag string can simply not be specified in the initial config
	conf.CTFd.Flags = conf.Lab.Flags()

	ctf, err := ctfdNew(conf.CTFd)
	if err != nil {
		return nil, err
	}

	guac, err := guacNew(conf.Guac)
	if err != nil {
		return nil, err
	}

	proxy, err := proxyNew(conf.Proxy, ctf, guac)
	if err != nil {
		return nil, err
	}

	ev := &event{
		ctfd:   ctf,
		guac:   guac,
		proxy:  proxy,
		labhub: labHub}

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
			Host:            hostIp,
			Port:            port,
			Name:            name,
			GuacUser:        auth.Username,
			Username:        &auth.Username,
			Password:        &auth.Password,
			EnableDrive:     true,
			DrivePath:       fmt.Sprintf("/tmp/%s", name),
			CreateDrivePath: true,
		}); err != nil {
			return nil, err
		}

	}

	return &auth, nil
}
