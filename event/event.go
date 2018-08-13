package event

import (
	"context"
	"fmt"
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
	"github.com/aau-network-security/go-ntp/svcs/revproxy"
	"github.com/google/uuid"
	"strings"
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
	Close() error
	Register(Group) Auth
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

	labHub, err := lab.NewHub(5, 10, *labConfig)
	if err != nil {
		return nil, err
	}

	// TODO: this is not implemented with dynamic flags in mind; dynamic flag string can simply not be specified in the initial config
	eventConfig.CTFd.Flags = labConfig.Flags()
	ctf, err := ctfd.New(eventConfig.CTFd)
	if err != nil {
		return nil, err
	}

	guac, err := guacamole.New(eventConfig.Guac)
	if err != nil {
		return nil, err
	}

	proxy, err := revproxy.New(eventConfig.RevProxy)
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
	ev.ctfd.Start(ctx)
	ev.guac.Start(ctx)
	ev.proxy.Start(ctx)
	return nil
}

func (ev *event) Close() error {
	ev.proxy.Close()
	ev.guac.Close()
	ev.ctfd.Close()
	ev.labhub.Close()
	return nil
}

func (ev *event) Register(group Group) Auth {
	auth := Auth{
		Username: rand(),
		Password: rand()}
	ev.guac.CreateUser(auth.Username, auth.Password)
	_, err := ev.labhub.Get()
	if err != nil {
		fmt.Println("Error while configuring lab for new group", err)
	}
	//lab.

	//ev.guac.CreateRDPConn(guacamole.CreateRDPConnOpts{
	//	Host: "localhost",
	//	Port:
	//})
	return auth
}
