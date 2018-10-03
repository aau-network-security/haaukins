package event

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

var (
	RdpConfErr      = errors.New("error too few rdp connections")
	StartingCtfdErr = errors.New("error while starting ctfd")
	StartingGuacErr = errors.New("error while starting guac")
	StartingRevErr  = errors.New("error while starting reverse proxy")
	EmptyNameErr    = errors.New("event requires a name")
	EmptyTagErr     = errors.New("event requires a tag")
	NoFrontendErr   = errors.New("lab requires at least one frontend")

	getDockerHostIp = docker.GetDockerHostIP
)

type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Group struct {
	Name string
	Lab  lab.Lab
}

type Event interface {
	Start(context.Context) error
	Close()
	Register(Group) (*Auth, error)
	Connect(*mux.Router)
	GetConfig() Config
	GetHub() lab.Hub
	GetGroups() []Group
}

type event struct {
	conf   Config
	ctfd   ctfd.CTFd
	guac   guacamole.Guacamole
	cbSrv  *callbackServer
	labhub lab.Hub
	groups []Group
}

func rand() string {
	return strings.Replace(fmt.Sprintf("%v", uuid.New()), "-", "", -1)
}

func New(conf Config) (Event, error) {
	if conf.Name == "" {
		return nil, EmptyNameErr
	}

	if conf.Tag == "" {
		return nil, EmptyTagErr
	}

	if len(conf.LabConfig.Frontends) == 0 {
		return nil, NoFrontendErr
	}

	if conf.VBoxLibrary == nil {
		conf.VBoxLibrary = vbox.NewLibrary(".")
	}

	for _, f := range conf.LabConfig.Frontends {
		if ok := conf.VBoxLibrary.IsAvailable(f); !ok {
			return nil, fmt.Errorf("Unknown frontend: %s", f)
		}
	}

	cb := &callbackServer{}
	if err := cb.Run(); err != nil {
		return nil, err
	}

	hub, err := lab.NewHub(conf.LabConfig, conf.VBoxLibrary, conf.Capacity, conf.Buffer)
	if err != nil {
		return nil, err
	}

	ctfdConf := ctfd.Config{
		Name:         conf.Name,
		CallbackHost: cb.host,
		CallbackPort: cb.port,
		Flags:        hub.Flags(),
	}

	ctf, err := ctfd.New(ctfdConf)
	if err != nil {
		return nil, err
	}

	guac, err := guacamole.New(guacamole.Config{})
	if err != nil {
		return nil, err
	}

	ev := &event{
		conf:   conf,
		labhub: hub,
		cbSrv:  cb,
		ctfd:   ctf,
		guac:   guac,
	}
	cb.event = ev

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

	return nil
}

func (ev *event) Close() {
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

	group.Lab = lab
	ev.groups = append(ev.groups, group)

	return &auth, nil
}

func (ev *event) Connect(r *mux.Router) {
	r.HandleFunc("/guacamole{rest:.*}", handler(ev.guac.ProxyHandler()))
	r.HandleFunc("/{rest:.*}", handler(ev.ctfd.ProxyHandler()))
}

func handler(h http.Handler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = mux.Vars(r)["rest"]
		h.ServeHTTP(w, r)
	}
}

func (ev *event) GetConfig() Config {
	return ev.conf
}

func (ev *event) GetHub() lab.Hub {
	return ev.labhub
}

func (ev *event) GetGroups() []Group {
	return ev.groups
}
