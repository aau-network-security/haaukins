package revproxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"math/rand"
	"time"
	"os"

	"github.com/aau-network-security/go-ntp/virtual/docker"
)

var (
	baseTmpl, _ = template.New("base").Parse(`
server {
    listen 80;

    {{range .Endpoints}}
    {{.}}
    {{end}}
}
`)
	AlreadyRunningErr = errors.New("Cannot add container when running")
)

type Config struct {
	Host string "yaml:host"
}

type Proxy interface {
	Start(context.Context) error
	Close() error
	Stop() error
	NumberOfEndpoints() int
}

type Connector interface {
	ConnectProxy() (docker.Identifier, string)
}

type nginx struct {
	cont      docker.Container
	confFile  string
	host      string
	running   bool
	endpoints []string
	aliasCont map[string]docker.Identifier
}

func New(conf Config, connectors ...Connector) (Proxy, error) {
	ng := &nginx{
		host:      conf.Host,
		aliasCont: make(map[string]docker.Identifier),
	}

	for _, c := range connectors {
        contId, conf := c.ConnectProxy()
		if err := ng.add(contId, conf); err != nil {
			return nil, err
		}
	}

	f, err := ioutil.TempFile("", "nginx-conf")
	if err != nil {
		return nil, err
	}

    confFile := f.Name()
    ng.confFile = confFile

	tmplCtx := struct {
		Endpoints []string
	}{
		ng.endpoints,
	}

	if err := baseTmpl.Execute(f, tmplCtx); err != nil {
		return nil, err
	}

	cConf := docker.ContainerConfig{
		Image: "nginx",
		EnvVars: map[string]string{
			"HOST": ng.host,
		},
		PortBindings: map[string]string{
			"443/tcp": "0.0.0.0:443",
			"80/tcp":  "0.0.0.0:80",
		},
		Mounts: []string{
			fmt.Sprintf("%s:/etc/nginx/conf.d/default.conf", confFile),
		},
		UseBridge: true,
	}

	c, err := docker.NewContainer(cConf)
	if err != nil {
		return nil, err
	}
	ng.cont = c

	for alias, cont := range ng.aliasCont {
		if err := c.Link(cont, alias); err != nil {
			return nil, err
		}
	}

	return ng, nil
}

func (ng *nginx) NumberOfEndpoints() int {
	return len(ng.endpoints)
}

func (ng *nginx) Start(ctx context.Context) error {
	if err := ng.cont.Start(); err != nil {
		return err
	}

	ng.running = true

	return nil
}

func (ng *nginx) add(c docker.Identifier, conf string) error {
	if ng.running {
		return AlreadyRunningErr
	}
	alias := randAlias(26)

	endpointTmpl, err := template.New(fmt.Sprintf("endpoint")).Parse(conf)
	if err != nil {
		return err
	}

	values := struct {
		Host string
	}{
		Host: alias,
	}
	var b bytes.Buffer
	if err := endpointTmpl.Execute(&b, values); err != nil {
		return err
	}

	ng.aliasCont[alias] = c
	ng.endpoints = append(ng.endpoints, b.String())

	return nil
}

const charset = "abcdefghijklmnopqrstuvwxyz"

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

func randAlias(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}


func (ng *nginx) Close() error {
	if err := os.Remove(ng.confFile); err != nil {
		return err
	}

	if err := ng.cont.Close(); err != nil {
		return err
	}

	return nil
}

func (ng *nginx) Stop() error {
	return ng.cont.Stop()
}
