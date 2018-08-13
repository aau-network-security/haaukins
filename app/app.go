package main

import (
	"context"
	"fmt"
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
	ev, err := event.New("app/config.yml", "app/exercises.yml")
	if err != nil {
		fmt.Println(err)
		return
	}
	handleCancel(func() {
		ev.Close()
	})

	err = ev.Start(context.TODO())
	if err != nil {
		fmt.Println(err)
		ev.Close()
		return
	}
	api := api.Api{Event: ev}
	api.RunServer("localhost", 8080)
}
