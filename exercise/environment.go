// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package exercise

import (
	"context"
	"io"
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
	Create(context.Context, bool) error
	Add(context.Context, ...[]store.Exercise) error
	ResetByTag(context.Context, string) error
	NetworkInterface() string
	LabSubnet() string
	LabDNS() string
	Challenges() []store.Challenge
	InstanceInfo() []virtual.InstanceInfo
	Start(context.Context) error
	StartExercises(context.Context, []store.Exercise) error
	StopExercises([]store.Exercise) error
	StatusExercise(store.Tag) []virtual.InstanceInfo
	Stop() error
	Suspend(ctx context.Context) error
	Resume(ctx context.Context) error
	io.Closer
}

type environment struct {
	tags      map[store.Tag]*exercise
	exercises []*exercise

	network    docker.Network
	dnsServer  *dns.Server
	dhcpServer *dhcp.Server
	dnsAddr    string
	lib        vbox.Library
	isVPN      bool
}

func NewEnvironment(lib vbox.Library) Environment {
	return &environment{
		tags: make(map[store.Tag]*exercise),
		lib:  lib,
	}
}

func (ee *environment) Create(ctx context.Context, isVPN bool) error {
	network, err := docker.NewNetwork(isVPN)
	if err != nil {
		return err
	}
	ee.network = network
	ee.network.SetIsVPN(ee.isVPN)
	ee.dnsAddr = ee.network.FormatIP(dns.PreferedIP)

	return nil
}

func (ee *environment) Add(ctx context.Context, confs ...[]store.Exercise) error {
	for _, step := range confs {
		for _, conf := range step {
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
	}
	return nil
}

func (ee *environment) StopExercises(exercises []store.Exercise) error {
	for _, ex := range ee.exercises {
		for _, t := range exercises {
			if t.Tags[0] == ex.tag {
				if err := ex.Stop(); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (ee *environment) NetworkInterface() string {
	return ee.network.Interface()
}
func (ee *environment) LabSubnet() string {
	return ee.dhcpServer.LabSubnet()
}

func (ee *environment) LabDNS() string {
	return ee.dhcpServer.LabDNS()
}

func (ee *environment) Start(ctx context.Context) error {

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

	return nil
}

func (ee *environment) StartExercises(ctx context.Context, exercises []store.Exercise) error {
	var res error
	var wg sync.WaitGroup
	for _, ex := range ee.exercises {
		for _, t := range exercises {
			if t.Tags[0] == ex.tag {
				wg.Add(1)
				go func(e *exercise) {
					if err := e.Start(ctx); err != nil && res == nil {
						res = err
					}
					wg.Done()
				}(ex)
			}
		}
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
		if err := e.Start(ctx); err != nil {
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

func (ee *environment) StatusExercise(exercise store.Tag) []virtual.InstanceInfo {
	for _, e := range ee.exercises {
		if e.tag == exercise {
			return e.InstanceInfo()
		}
	}
	return nil
}

func (ee *environment) refreshDNS(ctx context.Context) error {
	if ee.dnsServer != nil {
		if err := ee.dnsServer.Close(); err != nil {
			return err
		}
	}
	if ee.dhcpServer != nil {
		if err := ee.dhcpServer.Close(); err != nil {
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

	if err := serv.Run(ctx); err != nil {
		return err
	}

	if _, err := ee.network.Connect(serv.Container(), dns.PreferedIP); err != nil {
		return err
	}

	return nil
}
