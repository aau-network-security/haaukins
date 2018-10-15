// +build ignore

package lab_test

import (
	"fmt"
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"testing"
)

func TestLab(t *testing.T) {
	lib := vbox.NewLibrary("/scratch/events")

	exeConf := lab.Config{
		Exercises: []exercise.Config{{
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
		}},
	}

	fmt.Println(lab.NewLab(lib, exeConf))
}
