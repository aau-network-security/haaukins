package event

import (
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
	"github.com/aau-network-security/go-ntp/svcs/revproxy"
)

type Group struct {
	Name string
}

type Event struct {
	CTFd   ctfd.CTFd
	Proxy  revproxy.Proxy
	Guac   guacamole.Guacamole
	LabHub lab.Hub
}

func New(eventPath string, labPath string) (*Event, error) {
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

	ev := &Event{
		CTFd:   ctf,
		Guac:   guac,
		Proxy:  proxy,
		LabHub: labHub}

	err = ev.initialize()
	if err != nil {
		return nil, err
	}

	return ev, nil
}

func (ev *Event) initialize() error {
	ev.CTFd.ConnectProxy(ev.Proxy)
	ev.Guac.ConnectProxy(ev.Proxy)

	return nil
}

func (ev *Event) Start() error {
	// TODO: Start all components
	return nil
}

func (ev *Event) Close() error {
	ev.Proxy.Close()
	ev.Guac.Close()
	ev.CTFd.Close()
	ev.LabHub.Close()
	return nil
}

func (ev *Event) Register(group Group) error {
	// TODO: implement
	return nil
}
