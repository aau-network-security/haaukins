package ctfd

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"errors"

	"io"

	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs"
	"github.com/aau-network-security/haaukins/virtual"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var (
	NONCEREGEXP            = regexp.MustCompile(`csrf_nonce[ ]*=[ ]*"(.+)"`)
	ServerUnavailableErr   = errors.New("Server is unavailable")
	UserNotFoundErr        = errors.New("Could not find the specified user")
	CouldNotFindSessionErr = errors.New("Could not find the specified session")
	NoSessionErr           = errors.New("No session found")
	ChallengeNotFoundErr   = errors.New("Could not find the specified challenge")
	FlagNotFoundErr        = errors.New("Could not find the specified flag")
)

type CTFd interface {
	docker.Identifier
	io.Closer
	ProxyHandler(...func(*store.Team) error) svcs.ProxyConnector
	Start(context.Context) error
	Stop() error
	Flags() []store.FlagConfig
}

type Config struct {
	Name       string `yaml:"name"`
	AdminUser  string `yaml:"admin_user"`
	AdminEmail string `yaml:"admin_email"`
	AdminPass  string `yaml:"admin_pass"`
	Theme      string `yaml:"theme"`
	Flags      []store.FlagConfig
	Teams      []store.Team
}

type ctfd struct {
	conf     Config
	cont     docker.Container
	theme    Theme
	confDir  string
	nc       nonceClient
	users    []*user
	relation map[string]*user
	flagPool *FlagPool
}

type user struct {
	teamname string
	email    string
	password string
}

func New(ctx context.Context, conf Config) (CTFd, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	hc := &http.Client{
		Jar: jar,
	}
	nc := nonceClient{
		client: hc,
		port:   virtual.GetAvailablePort(),
	}

	if conf.Name == "" {
		conf.Name = "Demo"
	}

	if conf.Theme == "" {
		conf.Theme = "aau-survey"
	}

	if conf.AdminUser == "" {
		conf.AdminUser = "admin"
	}

	if conf.AdminEmail == "" {
		conf.AdminEmail = "admin@admin.com"
	}

	if conf.AdminPass == "" {
		pass := uuid.New().String()
		log.Info().
			Str("password", pass).
			Msg("setting new default password for ctfd")

		conf.AdminPass = pass
	}

	theme, ok := Themes[conf.Theme]
	if !ok {
		theme = Themes["aau"]
	}

	ctf := &ctfd{
		conf:     conf,
		theme:    theme,
		flagPool: NewFlagPool(),
		nc:       nc,
		relation: make(map[string]*user),
	}

	confDir, err := ioutil.TempDir("", "ctfd")
	if err != nil {
		return nil, err
	}

	ctf.confDir = confDir

	dconf := docker.ContainerConfig{
		Image: "registry.sec-aau.dk/aau/ctfd",
		Mounts: []string{
			fmt.Sprintf("%s/:/opt/CTFd/CTFd/data", confDir),
		},
		PortBindings: map[string]string{
			"8000/tcp": fmt.Sprintf("127.0.0.1:%d", ctf.nc.port),
		},
		UseBridge: true,
		Labels: map[string]string{
			"ntp": "ctfd",
		},
	}

	c := docker.NewContainer(dconf)
	err = c.Run(ctx)
	if err != nil {
		return nil, err
	}

	err = ctf.configureInstance()
	if err != nil {
		return nil, err
	}

	err = c.Stop()
	if err != nil {
		return nil, err
	}

	ctf.cont = c

	return ctf, nil

}

func (ctf *ctfd) Start(ctx context.Context) error {
	return ctf.cont.Start(ctx)
}

func (ctf *ctfd) Close() error {
	if err := os.RemoveAll(ctf.confDir); err != nil {
		return err
	}

	if err := ctf.cont.Close(); err != nil {
		return err
	}

	return nil
}

func (ctf *ctfd) Stop() error {
	return ctf.cont.Stop()
}

func (ctf *ctfd) Flags() []store.FlagConfig {
	return ctf.conf.Flags
}

func (ctf *ctfd) ID() string {
	return ctf.cont.ID()
}

func (ctf *ctfd) ProxyHandler(hooks ...func(*store.Team) error) svcs.ProxyConnector {
	origin, _ := url.Parse(ctf.nc.baseUrl())
	regOpts := []RegisterInterceptOpts{WithRegisterHooks(hooks...)}

	if ctf.theme.ExtraFields != nil {
		regOpts = append(regOpts, WithExtraRegisterFields(ctf.theme.ExtraFields))
	}

	return func(es store.EventFile) http.Handler {
		itc := svcs.Interceptors{
			NewRegisterInterception(es, regOpts...),
			NewCheckFlagInterceptor(es, ctf.flagPool),
			NewLoginInterceptor(es),
		}

		if ctf.theme.ExtraFields != nil {
			itc = append(itc, NewSignupInterception(ctf.theme.ExtraFields))
		}
		return itc.Intercept(httputil.NewSingleHostReverseProxy(origin))
	}
}

func (ctf *ctfd) configureInstance() error {
	endpoint := ctf.nc.baseUrl() + "/setup"

	if err := waitForServer(endpoint); err != nil {
		return err
	}

	nonce, err := ctf.nc.getNonce(endpoint)
	if err != nil {
		return err
	}

	form := url.Values{
		"ctf_name": {ctf.conf.Name},
		"name":     {ctf.conf.AdminUser},
		"password": {ctf.conf.AdminPass},
		"email":    {ctf.conf.AdminEmail},
		"nonce":    {nonce},
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := ctf.nc.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	if err := ctf.addTheme(ctf.theme); err != nil {
		return err
	}

	for id, flag := range ctf.conf.Flags {
		value := ctf.flagPool.AddFlag(flag, id+1)

		if err := ctf.createFlag(flag.Name, value, flag.Points); err != nil {
			return err
		}

		log.Debug().
			Str("name", flag.Name).
			Bool("static", flag.Static != "").
			Uint("points", flag.Points).
			Msg("Flag created")
	}

	for _, tt := range ctf.conf.Teams {
		jar, err := cookiejar.New(nil)
		if err != nil {
			return err
		}

		hc := &http.Client{
			Jar: jar,
		}

		t := team{
			nc: nonceClient{
				port:   ctf.nc.port,
				client: hc,
			},
			conf: tt,
		}

		if err := t.create(ctf.flagPool); err != nil {
			return err
		}
	}

	return nil
}

type nonceClient struct {
	port   uint
	client *http.Client
}

func (nc *nonceClient) baseUrl() string {
	return fmt.Sprintf("http://127.0.0.1:%d", nc.port)
}

func (nc *nonceClient) getNonce(path string) (string, error) {
	resp, err := nc.client.Get(path)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	matches := NONCEREGEXP.FindAllSubmatch(content, 1)
	if len(matches) == 0 {
		return "", fmt.Errorf("Unable to find nonce in page")
	}

	return string(matches[0][1]), nil
}

func (ctf *ctfd) createFlag(name, flag string, points uint) error {
	endpoint := ctf.nc.baseUrl() + "/admin/chal/new"

	nonce, err := ctf.nc.getNonce(endpoint)
	if err != nil {
		return err
	}

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	values := map[string]string{
		"name":         name,
		"value":        fmt.Sprintf("%d", points),
		"key":          flag,
		"nonce":        nonce,
		"key_type[0]":  "static",
		"category":     "",
		"description":  "",
		"max_attempts": "",
		"chaltype":     "standard",
	}

	for k, v := range values {
		err := w.WriteField(k, v)
		if err != nil {
			return err
		}
	}
	w.Close()

	req, err := http.NewRequest("POST", endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", w.FormDataContentType())

	resp, err := ctf.nc.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}

func (ctf *ctfd) addTheme(t Theme) error {
	endpoint := ctf.nc.baseUrl() + "/admin/config"

	nonce, err := ctf.nc.getNonce(endpoint)
	if err != nil {
		return err
	}

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	values := map[string]string{
		"nonce":         nonce,
		"ctf_name":      ctf.conf.Name,
		"ctf_logo":      "None",
		"ctf_theme":     "core",
		"css":           t.CSS,
		"mailfrom_addr": "",
		"mail_server":   "",
		"mail_port":     "",
		"mail_u":        "",
		"mail_p":        "",
		"mg_base_url":   "",
		"mg_api_key":    "",
		"start-month":   "",
		"start-day":     "",
		"start-year":    "",
		"start-hour":    "",
		"start-minute":  "",
		"start":         "",
		"end-month":     "",
		"end-day":       "",
		"end-year":      "",
		"end-hour":      "",
		"end-minute":    "",
		"end":           "",
		"freeze-month":  "",
		"freeze-day":    "",
		"freeze-year":   "",
		"freeze-hour":   "",
		"freeze-minute": "",
		"freeze":        "",
		"backup":        "",
	}

	for k, v := range values {
		err := w.WriteField(k, v)
		if err != nil {
			return err
		}
	}
	w.Close()

	req, err := http.NewRequest("POST", endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", w.FormDataContentType())

	resp, err := ctf.nc.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}

type team struct {
	nc    nonceClient
	conf  store.Team
	flags map[store.Tag]string
}

func (t *team) create(fp *FlagPool) error {
	endpoint := t.nc.baseUrl() + "/register"

	nonce, err := t.nc.getNonce(endpoint)
	if err != nil {
		return err
	}

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)

	values := map[string]string{
		"name":     t.conf.Name,
		"email":    t.conf.Email,
		"password": t.conf.HashedPassword,
		"nonce":    nonce,
	}

	for k, v := range values {
		err := w.WriteField(k, v)
		if err != nil {
			return err
		}
	}
	w.Close()

	req, err := http.NewRequest("POST", endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", w.FormDataContentType())

	resp, err := t.nc.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	for _, chal := range t.conf.SolvedChallenges {
		if err := t.solve(fp, chal.FlagTag); err != nil {
			return err
		}
	}

	return nil
}

func (t *team) solve(fp *FlagPool, tag store.Tag) error {
	id, err := fp.GetIdentifierByTag(tag)
	if err != nil {
		return err
	}

	flagval, err := fp.GetFlagByTag(tag)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/chal/%d", t.nc.baseUrl(), id)

	nonce, err := t.nc.getNonce(endpoint)
	if err != nil {
		return err
	}

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)

	values := map[string]string{
		"key":   flagval,
		"nonce": nonce,
	}

	for k, v := range values {
		err := w.WriteField(k, v)
		if err != nil {
			return err
		}
	}
	w.Close()

	req, err := http.NewRequest("POST", endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", w.FormDataContentType())

	resp, err := t.nc.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}

func waitForServer(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	errc := make(chan error, 1)
	poll := func() error {
		resp, err := http.Get(path)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
		}

		return nil
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				errc <- ServerUnavailableErr
				return
			default:
				err := poll()
				switch err.(type) {
				case net.Error:
					time.Sleep(time.Second)
					continue
				default:
					errc <- err
					return
				}
			}
		}
	}()

	return <-errc
}
