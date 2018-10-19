package exercise

import (
	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/svcs/dhcp"
	"github.com/aau-network-security/go-ntp/svcs/dns"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/rs/zerolog/log"
	"io"
	"sync"
)

type Environment interface {
	Add(conf store.Exercise, updateDNS bool) error
	ResetByTag(t string) error
	Interface() string
	Start() error
	Stop() error
	Restart() error
	io.Closer
}

type environment struct {
	tags      map[store.Tag]*exercise
	exercises []*exercise

	network    docker.Network
	dnsServer  *dns.Server
	dhcpServer *dhcp.Server
	dnsIP      string

	lib     vbox.Library
	closers []io.Closer
}

func NewEnvironment(lib vbox.Library, exercises ...store.Exercise) (Environment, error) {
	ee := &environment{
		tags: make(map[store.Tag]*exercise),
		lib:  lib,
	}

	var err error
	ee.network, err = docker.NewNetwork()
	if err != nil {
		return nil, err
	}

	ee.dhcpServer, err = dhcp.New(ee.network.FormatIP)
	if err != nil {
		return nil, err
	}

	if _, err := ee.network.Connect(ee.dhcpServer.Container(), 2); err != nil {
		return nil, err
	}

	// we need to set the DNS server BEFORE we add our exercises
	// else ee.dnsIP wil be "", and the resulting resolv.conf "nameserver "
	ee.dnsIP = ee.network.FormatIP(dns.PreferedIP)

	for _, e := range exercises {
		if err := ee.Add(e, false); err != nil {
			return nil, err
		}
	}

	if len(exercises) > 0 {
		if err := ee.updateDNS(); err != nil {
			return nil, err
		}
	}
	ee.closers = append(ee.closers, ee.dnsServer, ee.dhcpServer)

	return ee, nil
}

func (ee *environment) Add(conf store.Exercise, updateDNS bool) error {
	if len(conf.Tags) == 0 {
		return MissingTagsErr
	}

	for _, t := range conf.Tags {
		if _, ok := ee.tags[t]; ok {
			return DuplicateTagErr
		}
	}

	if updateDNS {
		if err := ee.updateDNS(); err != nil {
			return err
		}
	}

	e := &exercise{
		conf:       &conf,
		net:        ee.network,
		dnsIP:      ee.dnsIP,
		dockerHost: dockerHost{},
		lib:        ee.lib,
	}

	if err := e.Create(); err != nil {
		return err
	}

	for _, t := range conf.Tags {
		ee.tags[t] = e
	}
	ee.exercises = append(ee.exercises, e)
	ee.closers = append(ee.closers, e)

	return nil
}

func (ee *environment) Interface() string {
	return ee.network.Interface()
}

func (ee *environment) Start() error {
	if err := ee.dnsServer.Start(); err != nil {
		return err
	}

	if err := ee.dhcpServer.Start(); err != nil {
		return err
	}

	var res error
	var wg sync.WaitGroup
	for _, e := range ee.exercises {
		wg.Add(1)
		go func(e *exercise) {
			if err := e.Start(); err != nil && res == nil {
				res = err
			}
		}(e)
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

func (ee *environment) Restart() error {
	if err := ee.Stop(); err != nil {
		return err
	}
	if err := ee.Start(); err != nil {
		return err
	}

	return nil
}

func (ee *environment) Close() error {
	var wg sync.WaitGroup

	for _, closer := range ee.closers {
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

func (ee *environment) ResetByTag(s string) error {
	t, err := store.NewTag(s)
	if err != nil {
		return err
	}

	e, ok := ee.tags[t]
	if !ok {
		return UnknownTagErr
	}

	if err := e.Reset(); err != nil {
		return err
	}

	return nil
}

func (ee *environment) updateDNS() error {
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

	if _, err := ee.network.Connect(serv.Container(), dns.PreferedIP); err != nil {
		return err
	}

	ee.dnsServer = serv
	ee.dnsIP = ee.network.FormatIP(dns.PreferedIP)

	return nil
}
