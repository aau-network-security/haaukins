package exercise

import (
	"github.com/aau-network-security/go-ntp/svcs/dhcp"
	"github.com/aau-network-security/go-ntp/svcs/dns"
	"github.com/aau-network-security/go-ntp/virtual/docker"
)

type Environment struct {
	tags      map[string]*exercise
	exercises []*exercise

	network   *docker.Network
	dnsServer *dns.Server
	dnsIP     string
}

func NewEnvironment(exercises ...Config) (*Environment, error) {
	ee := &Environment{
		tags: make(map[string]*exercise),
	}

	var err error
	ee.network, err = docker.NewNetwork()
	if err != nil {
		return nil, err
	}

	dhcp, err := dhcp.New(ee.network.FormatIP)
	if err != nil {
		return nil, err
	}

	if _, err := ee.network.Connect(dhcp.Container(), 2); err != nil {
		return nil, err
	}

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

	return ee, nil
}

func (ee *Environment) Add(conf Config, updateDNS bool) error {
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
		conf:  &conf,
		net:   ee.network,
		dnsIP: ee.dnsIP,
	}

	if err := e.Start(); err != nil {
		return err
	}

	for _, t := range conf.Tags {
		ee.tags[t] = e
		ee.exercises = append(ee.exercises, e)
	}

	return nil
}

func (ee *Environment) ResetByTag(t string) error {
	e, ok := ee.tags[t]
	if !ok {
		return UnknownTagErr
	}

	if err := e.Reset(); err != nil {
		return err
	}

	return nil
}

func (ee *Environment) Interface() string {
	return ee.network.Interface()
}

func (ee *Environment) updateDNS() error {
	if ee.dnsServer != nil {
		if err := ee.dnsServer.Stop(); err != nil {
			return err
		}
	}

	var records []string
	for _, e := range ee.exercises {
		records = append(records, e.dnsRecords...)
	}

	serv, err := dns.New(records)
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
