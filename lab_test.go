package ntp_test

import (
	"fmt"
	"os"
	"testing"

	ntp "github.com/aau-network-security/go-ntp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestLab(t *testing.T) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	conf, err := ntp.LoadConfig("app/exercises.yml")

	fmt.Println("Err: ", err)
	econf := conf.ByTag("sql")

	eg := ntp.NewExerciseGroup([]*ntp.Exercise{{econf.DockerConfs}})

	fmt.Println(eg.Start())

}
