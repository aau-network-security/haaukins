package revproxy_test

import (
	"testing"
	"time"

	"github.com/rs/zerolog/log"
)

func TestRevProxy(t *testing.T) {
	for i := 0; i < 5; i++ {
		time.Sleep(500 * time.Millisecond)
		log.Info().Msg("hello world")
	}
	// g, _ := guacamole.New(
	// 	guacamole.Config{
	// 		AdminUser: "guacadmin",
	// 		AdminPass: "guacadmin2",
	// 	})

	// if err := g.Start(nil); err != nil {
	// 	fmt.Println(err)
	// 	return
	// }

	// defer g.Close()

	// c, _ := ctfd.New(
	// 	ctfd.Config{
	// 		Name:       "NTP",
	// 		AdminUser:  "admin",
	// 		AdminPass:  "admin",
	// 		AdminEmail: "admin@admin.dk",
	// 		Flags: []ctfd.Flag{{
	// 			Name:  "Test",
	// 			Flag:  "12345678",
	// 			Value: 15,
	// 		}},
	// 	})

	// if err := c.Start(nil); err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// defer c.Close()

	// p, _ := revproxy.New("localhost", g, c)

	// if err := p.Start(nil); err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// defer p.Close()

	// time.Sleep(60 * time.Second)
	// fmt.Println(g.CreateRDPConn(guacamole.CreateRDPConnOpts{
	// 	Name:     "thomas-conn5",
	// 	GuacUser: "thomas",
	// 	Host:     "sec02.lab.es.aau.dk",
	// 	Port:     4444,
	// }))
}
