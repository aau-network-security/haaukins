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
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/svcs/revproxy"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/rs/zerolog/log"
)

var (
	NONCEREGEXP = regexp.MustCompile(`csrf_nonce[ ]*=[ ]*"(.+)"`)
)

type CTFd interface {
	docker.Identifier
	revproxy.Connector
	Start(context.Context) error
	Close()
	Flags() []exercise.FlagConfig
}

type Config struct {
	Name       string `yaml:"name"`
	AdminUser  string `yaml:"admin_user"`
	AdminEmail string `yaml:"admin_email"`
	AdminPass  string `yaml:"admin_pass"`
	Flags      []exercise.FlagConfig
}

type ctfd struct {
	conf       Config
	cont       docker.Container
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

	return &ctfd{
		conf:       conf,
		httpclient: hc,
	}, nil

}

func (ctf *ctfd) Start(ctx context.Context) error {
	pwd, _ := os.Getwd()

	baseConf := &docker.ContainerConfig{
		Image: "aau/ctfd",
		Mounts: []string{
			fmt.Sprintf("%s/data:/opt/CTFd/CTFd/data", pwd),
		},
	}

	initConf := *baseConf
	initConf.PortBindings = map[string]string{
		"8000/tcp": "127.0.0.1:8000",
	}

	c, err := docker.NewContainer(initConf)
	if err != nil {
		return err
	}

	err = c.Start()
	if err != nil {
		return err
	}

	_, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err = ctf.configureInstance(ctx)
	if err != nil {
		return err
	}

	err = c.Close()
	if err != nil {
		return err
	}

	finalConf := *baseConf
	c, err = docker.NewContainer(finalConf)
	if err != nil {
		return err
	}
	ctf.cont = c

	err = c.Start()
	if err != nil {
		return err
	}

	return nil
}

func (ctf *ctfd) Close() {
	if ctf.cont != nil {
		ctf.cont.Close()
	}
}

func (ctf *ctfd) Flags() []exercise.FlagConfig {
	return ctf.conf.Flags
}

func (ctf *ctfd) ID() string {
	return ctf.cont.ID()
}

func (ctf *ctfd) ConnectProxy(p revproxy.Proxy) error {
	conf := `location / {
        proxy_pass http://{{.Host}}:8000/;
    }`

	return p.Add(ctf, conf)
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

func (ctf *ctfd) createFlag(ctx context.Context, name, flag string, points uint) error {
	endpoint := "http://localhost:8000" + "/admin/chal/new"

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

func (ctf *ctfd) configureInstance(ctx context.Context) error {
	endpoint := "http://localhost:8000/setup"

	if err := waitForServer(ctx, endpoint); err != nil {
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

	for _, f := range ctf.conf.Flags {
		log.Debug().Msgf("%s", f)
		err := ctf.createFlag(ctx, f.Name, f.Default, f.Points)
		if err != nil {
			log.Warn().Msgf("%s", err)
			return err
		}
		log.Debug().Msgf("Flag created")
	}

	return nil
}

func waitForServer(ctx context.Context, path string) error {
	errc := make(chan error, 1)
	poll := func() error {
		resp, err := http.Get(path)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Errorf("Unxpected status code: %d", resp.StatusCode)
		}

		return nil
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				errc <- ctx.Err()
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
