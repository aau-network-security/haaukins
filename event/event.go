package event

import (
	"context"
	"errors"
	"fmt"
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
	"github.com/aau-network-security/go-ntp/svcs/revproxy"
	"github.com/google/uuid"
	"strings"
)

var (
	RdpConfError = errors.New("Error ")

	ctfdNew   = ctfd.New
	guacNew   = guacamole.New
	proxyNew  = revproxy.New
	labNewHub = lab.NewHub
)

type Auth struct {
	Username string
	Password string
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

func New(eventPath string, labPath string) (Event, error) {
	eventConfig, err := loadConfig(eventPath)
	if err != nil {
		return nil, err
	}

	labConfig, err := lab.LoadConfig(labPath)
	if err != nil {
		return nil, err
	}

	labHub, err := labNewHub(2, 2, *labConfig)
	if err != nil {
		return nil, err
	}

	// TODO: this is not implemented with dynamic flags in mind; dynamic flag string can simply not be specified in the initial config
	eventConfig.CTFd.Flags = labConfig.Flags()

	ctf, err := ctfdNew(eventConfig.CTFd)
	if err != nil {
		return nil, err
	}

	guac, err := guacNew(eventConfig.Guac)
	if err != nil {
		return nil, err
	}

	proxy, err := proxyNew(eventConfig.RevProxy)
	if err != nil {
		return nil, err
	}

	ev := &event{
		ctfd:   ctf,
		guac:   guac,
		proxy:  proxy,
		labhub: labHub}

	err = ev.initialize()
	if err != nil {
		return nil, err
	}

	return ev, nil
}

func (ev *event) initialize() error {
	ev.ctfd.ConnectProxy(ev.proxy)
	ev.guac.ConnectProxy(ev.proxy)

	return nil
}

func (ev *event) Start(ctx context.Context) error {
	err := ev.ctfd.Start(ctx)
	if err != nil {
		return errors.New(fmt.Sprintf("Error while starting CTFD: %s", err))
	}

	err = ev.guac.Start(ctx)
	if err != nil {
		return errors.New(fmt.Sprintf("Error while starting Guacamole: %s", err))
	}

	err = ev.proxy.Start(ctx)
	if err != nil {
		return errors.New(fmt.Sprintf("Error while starting reverse proxy: %s", err))
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
		fmt.Println("Multiple RDP ports found while only one is supported, configuring first port by default.")
	} else if len(rdpConnPorts) == 0 {
		return nil, RdpConfError
	}

	auth := Auth{
		Username: rand(),
		Password: rand()}
	ev.guac.CreateUser(auth.Username, auth.Password)

	ev.guac.CreateRDPConn(guacamole.CreateRDPConnOpts{
		Host:     "localhost",
		Port:     rdpConnPorts[0],
		Name:     group.Name,
		GuacUser: auth.Username,
		Username: &auth.Username,
		Password: &auth.Password,
	})
	return &auth, nil
}
