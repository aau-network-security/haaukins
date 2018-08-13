package main

import (
	"context"
	"github.com/aau-network-security/go-ntp/api"
	"github.com/aau-network-security/go-ntp/event"
	"os"
	"os/signal"
	"syscall"
)

func handleCancel(clean func()) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		clean()
		os.Exit(1)
	}()
}

func main() {
	ev, _ := event.New("app/config.yml", "app/exercises.yml")
	handleCancel(func() {
		ev.Close()
	})

	ev.Start(context.TODO())
	api := api.Api{ev}
	api.RunServer("localhost", 8080)
}
