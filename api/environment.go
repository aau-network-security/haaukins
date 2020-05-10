package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/guacamole"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/rs/zerolog/log"
	"net/url"
	"strings"
	"time"
)

type environment struct {
	timer		*time.Time
	challenges 	[]string
	lab 		lab.Lab
	guacamole 	guacamole.Guacamole
	guacPort 	uint
}

func (e environment) Assign(client *ClientRequest) error {
	rdpPorts := e.lab.RdpConnPorts()
	if n := len(rdpPorts); n == 0 {
		log.
			Debug().
			Int("amount", n).
			Msg("Too few RDP connections")

		return errors.New("RdpConfErr")
	}
	fmt.Println(client.username)
	fmt.Println(client.password)
	u := guacamole.GuacUser{
		Username: client.username,
		Password: client.password,
	}

	if err := e.guacamole.CreateUser(u.Username, u.Password); err != nil {
		log.
			Debug().
			Str("err", err.Error()).
			Msg("Unable to create guacamole user")
		return err
	}

	dockerHost := docker.NewHost()
	hostIp, err := dockerHost.GetDockerHostIP()
	if err != nil {
		return err
	}

	for i, port := range rdpPorts {
		num := i + 1
		name := fmt.Sprintf("%s-client%d", client.username, num)

		log.Debug().Str("team", client.username).Uint("port", port).Msg("Creating RDP Connection for group")
		if err := e.guacamole.CreateRDPConn(guacamole.CreateRDPConnOpts{
			Host:     hostIp,
			Port:     port,
			Name:     name,
			GuacUser: u.Username,
			Username: &u.Username,
			Password: &u.Password,
		}); err != nil {
			return err
		}
	}

	content, err := e.guacamole.RawLogin(client.username, client.password)
	if err != nil {
		return err
	}
	fmt.Println(string(content))
	aa :=  url.QueryEscape(string(content))
	fmt.Println(aa)
	client.cookies[strings.Join(e.challenges, ",")] = aa
	client.ports[strings.Join(e.challenges, ",")] = e.guacPort
	return nil
}

func (e environment) Close() error {
	panic("implement me")
}

func (e environment) Start() error {
	panic("implement me")
}

type Environment interface {
	Assign(*ClientRequest) error
	Close() error
	Start() error
}

func newEnvironment(challenges []string) (Environment, error){

	//todo implement the challenges
	ctx := context.Background()
	ef, err := store.NewExerciseFile("/home/gian/Documents/haaukins_files/configs/exercises.yml")
	exercises := store.Tag("ftp")
	exer, err := ef.GetExercisesByTags(exercises)
	if err != nil {
		return environment{}, err
	}

	labConf := lab.Config{
		Exercises: exer,
		Frontends: []store.InstanceConfig{{
			Image: "kali",
			MemoryMB: uint(4096),
		}},
	}

	vlib := vbox.NewLibrary("/home/gian/Documents/ova")

	lh := lab.LabHost{
		Vlib: vlib,
		Conf: labConf,
	}

	guac, err := guacamole.New(ctx, guacamole.Config{})
	if err != nil {
		log.Error().Msgf("Error while creating new guacamole %s", err.Error())
		return environment{}, err
	}

	if err := guac.Start(ctx); err != nil {
		log.Error().Msgf("Error while starting guacamole %s", err.Error())
		return environment{}, err
	}

	lab, err := lh.NewLab(ctx)
	if err != nil {
		log.Error().Msgf("Error while creating new lab %s", err.Error())
		return environment{}, err
	}

	if err := lab.Start(ctx); err != nil {
		log.Error().Msgf("Error while starting lab %s", err.Error())
	}


	fmt.Println(guac.GetPort())
	env := &environment{
		timer:      nil, 		//todo implement the timer
		challenges: challenges,
		lab:        lab,
		guacamole:  guac,
		guacPort:   guac.GetPort(),
	}
	
	return env, nil
}