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

	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/svcs"
	"github.com/aau-network-security/go-ntp/virtual"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

var (
	NONCEREGEXP = regexp.MustCompile(`csrf_nonce[ ]*=[ ]*"(.+)"`)

	ServerUnavailableErr = errors.New("Server is unavailable")
)

type CTFd interface {
	docker.Identifier
	svcs.ProxyConnector
	Start() error
	Close() error
	Stop() error
	Flags() []store.FlagConfig
}

type Config struct {
	Name         string `yaml:"name"`
	AdminUser    string `yaml:"admin_user"`
	AdminEmail   string `yaml:"admin_email"`
	AdminPass    string `yaml:"admin_pass"`
	CallbackHost string
	CallbackPort uint
	Flags        []store.FlagConfig
}

type ctfd struct {
	conf       Config
	cont       docker.Container
	confDir    string
	port       uint
	httpclient *http.Client
}

func New(conf Config) (CTFd, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}

	hc := &http.Client{
		Jar: jar,
	}

	if conf.Name == "" {
		conf.Name = "Demo"
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

	ctf := &ctfd{
		conf:       conf,
		httpclient: hc,
		port:       virtual.GetAvailablePort(),
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
		EnvVars: map[string]string{
			"ADMIN_HOST": conf.CallbackHost,
			"ADMIN_PORT": fmt.Sprintf("%d", conf.CallbackPort),
		},
		PortBindings: map[string]string{
			"8000/tcp": fmt.Sprintf("127.0.0.1:%d", ctf.port),
		},
		UseBridge: true,
	}

	c, err := docker.NewContainer(dconf)
	if err != nil {
		return nil, err
	}

	err = c.Start()
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

func (ctf *ctfd) Start() error {
	return ctf.cont.Start()
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

func (ctf *ctfd) ProxyHandler() http.Handler {
	origin, _ := url.Parse(ctf.baseUrl())
	return httputil.NewSingleHostReverseProxy(origin)
}

func (ctf *ctfd) baseUrl() string {
	return fmt.Sprintf("http://127.0.0.1:%d", ctf.port)
}

func (ctf *ctfd) getNonce(path string) (string, error) {
	resp, err := ctf.httpclient.Get(path)
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
	endpoint := ctf.baseUrl() + "/admin/chal/new"

	nonce, err := ctf.getNonce(endpoint)
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

	resp, err := ctf.httpclient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	return nil
}

func (ctf *ctfd) configureInstance() error {
	endpoint := ctf.baseUrl() + "/setup"

	if err := waitForServer(endpoint); err != nil {
		return err
	}

	nonce, err := ctf.getNonce(endpoint)
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

	resp, err := ctf.httpclient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	for _, flag := range ctf.conf.Flags {
		err := ctf.createFlag(flag.Name, flag.Default, flag.Points)
		if err != nil {
			return err
		}
		log.Debug().
			Str("name", flag.Name).
			Str("flag", flag.Default).
			Uint("points", flag.Points).
			Msg("Flag created")
	}

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
