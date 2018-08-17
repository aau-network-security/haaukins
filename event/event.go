package event

import (
	"context"
	"errors"
	"fmt"
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
	"github.com/aau-network-security/go-ntp/svcs/revproxy"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"strings"
)

var (
	RdpConfError = errors.New("Error ")

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

	labHub, err := labNewHub(conf.Lab, "/scratch/git/training-platform/exercises")
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
		return errors.New(fmt.Sprintf("error while starting CTFD: %s", err))
	}

	if err := ev.guac.Start(ctx); err != nil {
		return errors.New(fmt.Sprintf("error while starting Guacamole: %s", err))
	}

	if err := ev.proxy.Start(ctx); err != nil {
		return errors.New(fmt.Sprintf("error while starting reverse proxy: %s", err))
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

	rdpConnPorts := lab.RdpConnPorts()

	if len(rdpConnPorts) > 1 {
		log.Debug().Msgf("Multiple RDP ports found while only one is supported, configuring first port by default.")
	} else if len(rdpConnPorts) == 0 {
		return nil, RdpConfError
	}

	auth := Auth{
		Username: rand(),
		Password: rand()}
	if err := ev.guac.CreateUser(auth.Username, auth.Password); err != nil {
		return nil, err
	}

	hostIp, err := getDockerHostIp()
	if err != nil {
		return nil, err
	}

	if err := ev.guac.CreateRDPConn(guacamole.CreateRDPConnOpts{
		Host:     hostIp,
		Port:     rdpConnPorts[0],
		Name:     group.Name,
		GuacUser: auth.Username,
		Username: &auth.Username,
		Password: &auth.Password,
	}); err != nil {
		return nil, err
	}
	return &auth, nil
}
