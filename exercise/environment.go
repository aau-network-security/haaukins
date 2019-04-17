package exercise

import (
	"context"
	"io"
	"sync"

	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/dhcp"
	"github.com/aau-network-security/haaukins/svcs/dns"
	"github.com/aau-network-security/haaukins/virtual"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/rs/zerolog/log"
)

type Environment interface {
	Create(context.Context) error
	Add(context.Context, ...store.Exercise) error
	ResetByTag(context.Context, string) error
	NetworkInterface() string
	Challenges() []store.Challenge
	InstanceInfo() []virtual.InstanceInfo
	Start(context.Context) error
	Stop() error
	io.Closer
}

type environment struct {
	tags      map[store.Tag]*exercise
	exercises []*exercise

	network    docker.Network
	dnsServer  *dns.Server
	dhcpServer *dhcp.Server
	dnsAddr    string

	lib vbox.Library
}

func NewEnvironment(lib vbox.Library) Environment {
	return &environment{
		tags: make(map[store.Tag]*exercise),
		lib:  lib,
	}
}

func (ee *environment) Create(ctx context.Context) error {
	network, err := docker.NewNetwork()
	if err != nil {
		return err
	}
	ee.network = network
	ee.dnsAddr = ee.network.FormatIP(dns.PreferedIP)

	return nil
}

func (ee *environment) Add(ctx context.Context, confs ...store.Exercise) error {
	for _, conf := range confs {
		if len(conf.Tags) == 0 {
			return MissingTagsErr
		}

		for _, t := range conf.Tags {
			if _, ok := ee.tags[t]; ok {
				return DuplicateTagErr
			}
		}

		e := NewExercise(conf, dockerHost{}, ee.lib, ee.network, ee.dnsAddr)
		if err := e.Create(ctx); err != nil {
			return err
		}

		for _, t := range conf.Tags {
			ee.tags[t] = e
		}

		ee.exercises = append(ee.exercises, e)
	}

	return ee.refreshDNS(ctx)
}

func (ee *environment) NetworkInterface() string {
	return ee.network.Interface()
}

func (ee *environment) Start(ctx context.Context) error {
	var err error
	ee.dhcpServer, err = dhcp.New(ee.network.FormatIP)
	if err != nil {
		return err
	}

	if err := ee.dhcpServer.Run(ctx); err != nil {
		return err
	}

	if _, err := ee.network.Connect(ee.dhcpServer.Container(), 2); err != nil {
		return err
	}

	var res error
	var wg sync.WaitGroup
	for _, ex := range ee.exercises {
		wg.Add(1)
		go func(e *exercise) {
			if err := e.Start(ctx); err != nil && res == nil {
				res = err
			}
			wg.Done()
		}(ex)
	}
	wg.Wait()

	return res
}

func (ee *environment) Stop() error {
	if err := ee.dnsServer.Stop(); err != nil {
		return err
	}

	if err := ee.dhcpServer.Stop(); err != nil {
		return err
	}

	for _, e := range ee.exercises {
		if err := e.Stop(); err != nil {
			return err
		}
	}

	return nil
}

func (ee *environment) Close() error {
	var wg sync.WaitGroup

	var closers []io.Closer
	if ee.dhcpServer != nil {
		closers = append(closers, ee.dhcpServer)
	}

	if ee.dnsServer != nil {
		closers = append(closers, ee.dnsServer)
	}

	for _, e := range ee.exercises {
		closers = append(closers, e)
	}

	for _, closer := range closers {
		wg.Add(1)
		go func(c io.Closer) {
			if err := c.Close(); err != nil {
				log.Warn().Msgf("error while closing environment: %s", err)
			}
			wg.Done()
		}(closer)

	}
	wg.Wait()

	if err := ee.network.Close(); err != nil {
		log.Warn().Msgf("error while closing environment: %s", err)
	}

	return nil
}

func (ee *environment) ResetByTag(ctx context.Context, s string) error {
	t, err := store.NewTag(s)
	if err != nil {
		return err
	}

	e, ok := ee.tags[t]
	if !ok {
		return UnknownTagErr
	}

	if err := e.Reset(ctx); err != nil {
		return err
	}

	return nil
}

func (ee *environment) Challenges() []store.Challenge {
	var challenges []store.Challenge
	for _, e := range ee.exercises {
		challenges = append(challenges, e.Challenges()...)
	}

	return challenges
}

func (ee *environment) InstanceInfo() []virtual.InstanceInfo {
	var instances []virtual.InstanceInfo
	for _, e := range ee.exercises {
		instances = append(instances, e.InstanceInfo()...)
	}
	return instances
}

func (ee *environment) refreshDNS(ctx context.Context) error {
	if ee.dnsServer != nil {
		if err := ee.dnsServer.Close(); err != nil {
			return err
		}
	}

	var rrSet []dns.RR
	for _, e := range ee.exercises {
		for _, record := range e.dnsRecords {
			rrSet = append(rrSet, dns.RR{record.Name, record.Type, record.RData})
		}
	}

	serv, err := dns.New(rrSet)
	if err != nil {
		return err
	}
	ee.dnsServer = serv

	if err := serv.Run(ctx); err != nil {
		return err
	}

	if _, err := ee.network.Connect(serv.Container(), dns.PreferedIP); err != nil {
		return err
	}

	return nil
}
