package api

import (
	"github.com/aau-network-security/go-ntp/event"
)

type service struct {
	events map[string]event.Event
}

type Service interface {
	CreateEvent(string, string, []string) error
	StopEvent(string) error
}

func (svc *service) CreateEvent(name, tag string, etags []string) error {
	return nil
}
