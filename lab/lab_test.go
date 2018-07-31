package lab_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func TestLab(t *testing.T) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	lib := vbox.NewLibrary("/scratch/events")

	exeConf := []exercise.Config{{
		Name: "SQL",
		Tags: []string{"sql"},
		DockerConfs: []exercise.DockerConfig{
			{
				Image: "aau/sql-server",
				Records: []exercise.RecordConfig{
					{
						Name: "netsec-forum.dk",
						Type: "A",
					},
				},
			},
		},
	}}

	fmt.Println(lab.NewLab(lib, exeConf))
}
