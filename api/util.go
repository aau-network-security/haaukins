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
)

const waitingHTMLTemplate = `
<html lang="en" dir="ltr">
		  <meta http-equiv="refresh" content="60" />
		  <head>
			<style>
				html, body {
		  height: 100%;
		  width: 100%;
		  margin: 0;
		  padding: 0;
		  font-size: 100%;
		  background: #191a1a;
		  text-align: center;
		}
		
		h1 {
		  margin: 100px;
		  padding: 0;
		  font-family: ‘Arial Narrow’, sans-serif;
		  font-weight: 100;
		  font-size: 1.1em;
		  color: #a3e1f0;
		}
		h2 {
		  margin:50px;
		  color: #a3e1f0;
		  font-family: ‘Arial Narrow’, sans-serif;
		}
		
		span {
		  position: relative;
		  top: 0.63em;  
		  display: inline-block;
		  text-transform: uppercase;  
		  opacity: 0;
		  transform: rotateX(-90deg);
		}
		
		.let1 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.2s;
		}
		
		.let2 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.3s;
		}
		
		.let3 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.4s;
		}
		
		.let4 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.5s;
		
		}
		
		.let5 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.6s;
		}
		
		.let6 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.7s;
		}
		
		.let7 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.8s;
		}
		
		@keyframes drop {
			10% {
				opacity: 0.5;
			}
			20% {
				opacity: 1;
				top: 3.78em;
				transform: rotateX(-360deg);
			}
			80% {
				opacity: 1;
				top: 3.78em;
				transform: rotateX(-360deg);
			}
			90% {
				opacity: 0.5;
			}
			100% {
				opacity: 0;
				top: 6.94em
			}
		}
    </style>
  </head>
  <body>
  <h1>
    <span class="let1">l</span>  
    <span class="let2">o</span>  
    <span class="let3">a</span>  
    <span class="let4">d</span>  
    <span class="let5">i</span>  
    <span class="let6">n</span>  
    <span class="let7">g</span>  
  </h1>
<h2>
Virtualized Environment
</h2>
  </body>
</html>
`

func CreateGuacamole() (string, uint, error){

	ctx := context.Background()
	ef, err := store.NewExerciseFile("/home/gian/Documents/haaukins_files/configs/exercises.yml")
	exercises := store.Tag("ftp")
	exer, err := ef.GetExercisesByTags(exercises)
	if err != nil {
		return "", 0, err
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
		return "", 0, err
	}

	if err := guac.Start(ctx); err != nil {
		return "", 0, err
	}

	labb, err := lh.NewLab(ctx)
	if err != nil {
		return "", 0, errors.New("error creating lab")
	}
	if err := labb.Start(ctx); err != nil {
		log.Error().Msgf("Error while starting lab %s", err.Error())
	}
	if err := AssignLab("team", labb, guac); err != nil {
		fmt.Println("Issue assigning lab: ", err)
		return "", 0, err
	}
	content, err := guac.RawLogin("team", "team")
	if err != nil {
		return "", 0, err
	}

	return url.QueryEscape(string(content)), guac.GetPort(), nil
}

func AssignLab(user string, lab lab.Lab, guac guacamole.Guacamole) error{
	rdpPorts := lab.RdpConnPorts()
	if n := len(rdpPorts); n == 0 {
		log.
			Debug().
			Int("amount", n).
			Msg("Too few RDP connections")

		return errors.New("RdpConfErr")
	}
	u := guacamole.GuacUser{
		Username: user,
		Password: user,
	}

	if err := guac.CreateUser(u.Username, u.Password); err != nil {
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
		name := fmt.Sprintf("%s-client%d", user, num)

		log.Debug().Str("team", user).Uint("port", port).Msg("Creating RDP Connection for group")
		if err := guac.CreateRDPConn(guacamole.CreateRDPConnOpts{
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

	return nil
}