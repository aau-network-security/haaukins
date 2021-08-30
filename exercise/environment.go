// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package exercise

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/aau-network-security/haaukins/network/dhcp"
	"github.com/aau-network-security/haaukins/network/dns"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/virtual"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/rs/zerolog/log"
)

type Environment interface {
	Create(context.Context, int32) error
	Add(context.Context, ...store.Exercise) error
	ResetByTag(context.Context, string) error
	Reset(context.Context) error
	NetworkInterface() string
	LabSubnet() string
	LabDNS() string
	DNSRecords() []*DNSRecord
	Challenges() []store.Challenge
	InstanceInfo() []virtual.InstanceInfo
	Start(context.Context) error
	StartByTag(context.Context, string) error
	StopByTag(string) error
	Stop() error
	Suspend(ctx context.Context) error
	Resume(ctx context.Context) error
	SetDisabledExercises([]store.Tag)
	io.Closer
}

type DNSRecord struct {
	Record map[string]string
}

type environment struct {
	tags              map[store.Tag]*exercise
	exercises         []*exercise
	disabledExercises []store.Tag // this is parent tag of exercises to be disabled
	dnsrecords        []*DNSRecord
	network           docker.Network
	dnsServer         *dns.Server
	dhcpServer        *dhcp.Server
	dnsAddr           string
	lib               vbox.Library
	isVPN             bool
}

func NewEnvironment(lib vbox.Library) Environment {
	return &environment{
		tags: make(map[store.Tag]*exercise),
		lib:  lib,
	}
}

func (ee *environment) SetDisabledExercises(disabledExs []store.Tag) {
	log.Debug().Msgf("Setting Disabled Exercises %v", disabledExs)
	ee.disabledExercises = disabledExs
}

func (ee *environment) Create(ctx context.Context, isVPN int32) error {
	network, err := docker.NewNetwork(isVPN)
	if err != nil {
		return fmt.Errorf("docker new network err %v", err)
	}
	ee.network = network
	ee.network.SetIsVPN(isVPN)
	ee.dnsAddr = ee.network.FormatIP(dns.PreferedIP)

	return nil
}

func (ee *environment) Add(ctx context.Context, confs ...store.Exercise) error {

	var e *exercise
	var aRecord string

	for _, conf := range confs {
		if conf.Tag == "" {
			return MissingTagsErr
		}

		if _, ok := ee.tags[conf.Tag]; ok {
			return DuplicateTagErr
		}

		if conf.Static {
			e = NewExercise(conf, dockerHost{}, nil, nil, "")
			log.Debug().
				Str("Challenge Name", conf.Name).
				Str("Challenge Category", conf.Category).
				Bool("Is Secret", conf.Secret).
				Msgf("Configuring the static challenge")
		} else {
			e = NewExercise(conf, dockerHost{}, ee.lib, ee.network, ee.dnsAddr)
			if err := e.Create(ctx); err != nil {
				return err
			}
			ip := strings.Split(e.dnsAddr, ".")

			for i, c := range e.containerOpts {
				for _, r := range c.Records {
					if strings.Contains(c.DockerConf.Image, "client") {
						continue
					}
					if r.Type == "A" {
						aRecord = r.Name
						ee.dnsrecords = append(ee.dnsrecords, &DNSRecord{Record: map[string]string{
							fmt.Sprintf("%s.%s.%s.%d", ip[0], ip[1], ip[2], e.ips[i]): aRecord,
						}})
					}
				}
			}
		}
		ee.tags[conf.Tag] = e
		ee.exercises = append(ee.exercises, e)
	}

	return nil
}

func (ee *environment) NetworkInterface() string {
	return ee.network.Interface()
}
func (ee *environment) LabSubnet() string {
	return ee.dhcpServer.LabSubnet()
}

func (ee *environment) DNSRecords() []*DNSRecord {
	return ee.dnsrecords
}

func (ee *environment) LabDNS() string {
	return ee.dhcpServer.LabDNS()
}

func (ee *environment) Start(ctx context.Context) error {
	allExercises := ee.exercises
	var enabledExercies []*exercise
	disabledExercises := ee.disabledExercises
	// disabledExercises : will be created but will not be started
	for _, e := range allExercises {
		if contains(disabledExercises, e.tag) {
			continue
		}
		enabledExercies = append(enabledExercies, e)
	}

	if err := ee.refreshDNS(ctx); err != nil {
		log.Error().Err(err).Msg("Refreshing DNS error")
		return err
	}

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
	for _, ex := range enabledExercies {
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

func (ee *environment) Reset(ctx context.Context) error {

	if err := ee.refreshDHCP(ctx); err != nil {
		return err
	}

	if err := ee.refreshDNS(ctx); err != nil {
		return err
	}
	for _, e := range ee.exercises {
		if err := e.Reset(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (ee *environment) Suspend(ctx context.Context) error {
	for _, e := range ee.exercises {
		if err := e.Suspend(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Resume will unpause paused containers
// and start suspended vms (saved state vms)
func (ee *environment) Resume(ctx context.Context) error {
	for _, e := range ee.exercises {
		for _, ins := range e.InstanceInfo() {
			if ins.State == virtual.Suspended {
				if err := e.Start(ctx); err != nil {
					return err
				}
			}
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

func (ee *environment) StartByTag(ctx context.Context, tag string) error {
	t, err := store.NewTag(tag)
	if err != nil {
		return err
	}
	e, ok := ee.tags[t]
	if !ok {
		return UnknownTagErr
	}
	if err := e.Start(ctx); err != nil {
		return err
	}
	return nil
}

func (ee *environment) StopByTag(tag string) error {
	t, err := store.NewTag(tag)
	if err != nil {
		return err
	}
	e, ok := ee.tags[t]
	if !ok {
		return UnknownTagErr
	}
	if err := e.Stop(); err != nil {
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
			rrSet = append(rrSet, dns.RR{Name: record.Name, Type: record.Type, RData: record.RData})
		}
	}

	serv, err := dns.New(rrSet)
	if err != nil {
		return err
	}
	ee.dnsServer = serv

	if err := ee.dnsServer.Run(ctx); err != nil {
		return err
	}

	if _, err := ee.network.Connect(ee.dnsServer.Container(), dns.PreferedIP); err != nil {
		return err
	}

	return nil
}

func (ee *environment) refreshDHCP(ctx context.Context) error {
	if ee.dhcpServer != nil {
		if err := ee.dhcpServer.Close(); err != nil {
			return err
		}
	}

	serv, err := dhcp.New(ee.network.FormatIP)
	if err != nil {
		return err
	}
	ee.dhcpServer = serv

	if err := ee.dhcpServer.Run(ctx); err != nil {
		return err
	}

	if _, err := ee.network.Connect(ee.dhcpServer.Container(), 2); err != nil {
		return err
	}

	return nil
}

func contains(l []store.Tag, t store.Tag) bool {
	for _, v := range l {
		if v == t {
			return true
		}
	}
	return false
}
