// +build ignore

package guacamole_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/aau-network-security/go-ntp/svcs/guacamole"
)

func TestGuacamole(t *testing.T) {
	// docker.Registries["registry.sec-aau.dk"] = dockerclient.AuthConfiguration{
	// 	Username:      "travis",
	// 	Password:      "T5CBgSJfMfJfUAyreMBHz96DhFE89ADK",
	// 	ServerAddress: "registry.sec-aau.dk",
	// }

	g, err := guacamole.New(
		guacamole.Config{
			AdminPass: "guacadmin",
		})

	if err != nil {
		fmt.Println(err)
		return
	}

	i := 1
	for {

		username := fmt.Sprintf("tkp%d", i)
		fmt.Println(username)
		fmt.Println(g.CreateUser(username, "tkp"))

		g.Logout()

		i += 1

		time.Sleep(10 * time.Second)
	}

	// fmt.Println(g.CreateRDPConn(guacamole.CreateRDPConnOpts{
	// 	Name:     "thomas-conn5",
	// 	GuacUser: "thomas",
	// 	Host:     "sec02.lab.es.aau.dk",
	// 	Port:     4444,
	// }))
}
