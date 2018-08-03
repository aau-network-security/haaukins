package event

import (
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
	"github.com/aau-network-security/go-ntp/svcs/revproxy"
)

type Event struct {
	CTFd   ctfd.CTFd
	Proxy  revproxy.Proxy
	Guac   guacamole.Guacamole
	LabHub lab.Hub
}

func New(path string) (*Event, error) {
	config, err := loadConfig(path)
	if err != nil {
		return nil, err
	}

	ctf, err := ctfd.New(config.CTFd)
	if err != nil {
		return nil, err
	}

	guac, err := guacamole.New(config.Guac)
	if err != nil {
		return nil, err
	}

	proxy, err := revproxy.New(config.RevProxy)
	if err != nil {
		return nil, err
	}

	ev := &Event{
		CTFd:  ctf,
		Guac:  guac,
		Proxy: proxy}

	ev.initialize("app/exercises.yml")

	return ev, nil
}

func (es *Event) initialize(path string) error {
	config, err := lab.LoadConfig(path)
	if err != nil {
		return err
	}
	hub, _ := lab.NewHub(10, 50, config)
	es.LabHub = hub

	es.CTFd.ConnectProxy(es.Proxy)
	es.Guac.ConnectProxy(es.Proxy)

	return nil
}

func (es *Event) Start() error {
	return nil
}

func (es *Event) Kill() error {
	return nil
}
