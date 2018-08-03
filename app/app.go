package main

import (
	"fmt"
	"github.com/aau-network-security/go-ntp/event"
)

func spawn() *event.Event {
	es, _ := event.New("app/config.yml", "app/exercises.yml")
	return es
}

func main() {
	event := spawn()
	fmt.Println("%+v", event)
}
