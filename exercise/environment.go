package exercise

import (
	"github.com/aau-network-security/go-ntp/svcs/dhcp"
	"github.com/aau-network-security/go-ntp/svcs/dns"
	"github.com/aau-network-security/go-ntp/virtual/docker"
)

type Environment interface {
	Add(conf Config, updateDNS bool) error
	ResetByTag(t string) error
	Interface() string
	Start() error
	Close() error
}

type environment struct {
	tags      map[string]*exercise
	exercises []*exercise

	network    docker.Network
	dnsServer  *dns.Server
	dhcpServer *dhcp.Server
	dnsIP      string
}

func NewEnvironment(exercises ...Config) (Environment, error) {
	ee := &environment{
		tags: make(map[string]*exercise),
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

	return ee, nil
}

func (ee *environment) Add(conf Config, updateDNS bool) error {
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

	if err := e.Create(); err != nil {
		return err
	}

	for _, t := range conf.Tags {
		ee.tags[t] = e
	}
	ee.exercises = append(ee.exercises, e)

	return nil
}

func (ee *environment) ResetByTag(t string) error {
	e, ok := ee.tags[t]
	if !ok {
		return UnknownTagErr
	}

	if err := e.Reset(); err != nil {
		return err
	}

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

	for _, e := range ee.exercises {
		if err := e.Start(); err != nil {
			return err
		}
	}

	return nil
}

func (ee *environment) Close() error {
	if err := ee.dnsServer.Close(); err != nil {
		return err
	}

	if err := ee.dhcpServer.Close(); err != nil {
		return err
	}

	for _, e := range ee.exercises {
		if err := e.Close(); err != nil {
			return err
		}
	}

	if err := ee.network.Close(); err != nil {
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
			rrSet = append(rrSet, dns.RR{record.Type, record.Name, record.RData})
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
