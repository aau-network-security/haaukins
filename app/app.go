package main

import (
	"github.com/aau-network-security/go-ntp/api"
	"github.com/aau-network-security/go-ntp/event"
)

func main() {
	ev, _ := event.New("app/config.yml", "app/exercises.yml")
	api := api.Api{Event: *ev}
	api.RunServer("localhost", 8080)
}
