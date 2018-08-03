package main

import (
	"fmt"
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
	"github.com/aau-network-security/go-ntp/svcs/revproxy"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type Config struct {
	CTFd     ctfd.Config      "yaml:ctfd"
	Guac     guacamole.Config "yaml:guacamole"
	RevProxy RevProxy         "yaml:revproxy"
}

//type CTFd struct {
//	Name       string "yaml:name"
//	AdminUser  string "yaml:admin_user"
//	AdminEmail string "yaml:admin_email"
//	AdminPass  string "yaml:admin_pass"
//}

/*type Guac struct {
	AdminUser string "yaml:admin_user"
	AdminPass string "yaml:admin_pass"
	Host string "yaml:host"
	Port uint "yaml:port"
}*/

type RevProxy struct {
	Host string "yaml:host"
}

type event struct {
	CTFd   ctfd.CTFd
	Proxy  revproxy.Proxy
	Guac   guacamole.Guacamole
	LabHub lab.Hub
}

func NewEvent(path string) (*event, error) {
	config, err := loadConfig(path)
	if err != nil {
		return nil, err
	}

	//ctfdConfig := &ctfd.Config{
	//	Name: config.CTFd.Name,
	//	AdminUser: config.CTFd.AdminUser,
	//	AdminEmail: config.CTFd.AdminEmail,
	//	AdminPass: config.CTFd.AdminPass,
	//	Flags: config.CTFd.Flags}

	ctf, err := ctfd.New(config.CTFd)
	if err != nil {
		return nil, err
	}

	//guacConfig := &guacamole.Config{
	//	AdminUser: config.Guac.AdminUser,
	//	AdminPass: config.Guac.AdminPass,
	//	Host: config.Guac.Host,
	//	Port: config.Guac.Port}

	guac, err := guacamole.New(config.Guac)

	if err != nil {
		return nil, err
	}

	proxy, err := revproxy.New(config.RevProxy.Host)

	if err != nil {
		return nil, err
	}

	ev := &event{
		CTFd:  ctf,
		Guac:  guac,
		Proxy: proxy}

	ev.initialize("app/exercises.yml")
	ev.start()

	return ev, nil
}

func (es *event) initialize(path string) error {
	es.CTFd.ConnectProxy(es.Proxy)
	es.Guac.ConnectProxy(es.Proxy)
	_, err := exercise.LoadConfig(path)
	if err != nil {
		return err
	}
	//lab.NewHub(10, 50, )
	return nil
}

func (es *event) start() error {
	return nil
}

func (es *event) stop() error {
	return nil
}

func loadConfig(path string) (*Config, error) {
	var config *Config
	rawData, err := ioutil.ReadFile(path)
	if err != nil {
		return config, err
	}

	if err := yaml.Unmarshal(rawData, &config); err != nil {
		return config, err
	}

	return config, nil
}

func spawn() *event {
	es, _ := NewEvent("app/config.yml")
	return es
}

func main() {
	//exConfig, err := exercise.LoadConfig("app/exercises.yml")
	//if err != nil {
	//	fmt.Print(err)
	//	return
	//}

	//lab.NewHub(5, 10 , exConfig.GetExercises())
	event := spawn()
	fmt.Println("%+v", event)
}
