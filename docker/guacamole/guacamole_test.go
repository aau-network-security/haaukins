package guacamole_test

import (
	"testing"
	"time"

	"github.com/aau-network-security/go-ntp/docker/guacamole"
)

func TestGuacamole(t *testing.T) {
	g, _ := guacamole.New(
		guacamole.Config{
			AdminUser: "guacadmin",
			AdminPass: "guacadmin2",
		})

	g.Start(nil)
	defer g.Close()

	time.Sleep(29 * time.Second)

	// fmt.Println(g.CreateRDPConn(guacamole.CreateRDPConnOpts{
	// 	Name:     "thomas-conn5",
	// 	GuacUser: "thomas",
	// 	Host:     "sec02.lab.es.aau.dk",
	// 	Port:     4444,
	// }))
}
