package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aau-network-security/go-ntp/event"
	"github.com/aau-network-security/go-ntp/exercise"
	"github.com/aau-network-security/go-ntp/lab"
	"github.com/aau-network-security/go-ntp/virtual/vbox"
	"github.com/gorilla/mux"
)

var (
	DuplicateEventErr = errors.New("event with that tag already exists")
	UnknownEventErr   = errors.New("unable to find event by that tag")
)

type service struct {
	events          map[string]event.Event
	exerciseLib     *exercise.Library
	frontendLibrary vbox.Library

	mux *mux.Router
}

type Service interface {
	CreateEvent(CreateEventRequest) error
	StopEvent(StopEventRequest) error
	Close()
}

func NewService() (Service, error) {
	elib, err := exercise.NewLibrary("exercises.yml")
	if err != nil {
		return nil, err
	}

	dir, _ := os.Getwd()
	vlib := vbox.NewLibrary(filepath.Join(dir, "vbox"))

	m := mux.NewRouter()
	go func() {
		if err := http.ListenAndServe(":8080", m); err != nil {
			fmt.Println("Serving error", err)
		}
	}()

	return &service{
		exerciseLib:     elib,
		events:          map[string]event.Event{},
		frontendLibrary: vlib,
		mux:             m,
	}, nil
}

type CreateEventRequest struct {
	Name         string   `json:"name"`
	Tag          string   `json:"tag"`
	ExerciseTags []string `json:"exercises"`
	Frontends    []string `json:"frontends"`
	Buffer       int      `json:"buffer"`
	Capacity     int      `json:"capacity"`
}

func (svc *service) CreateEvent(req CreateEventRequest) error {
	_, ok := svc.events[req.Tag]
	if ok {
		return DuplicateEventErr
	}

	var exer []exercise.Config
	var err error
	if len(req.ExerciseTags) > 0 {
		exer, err = svc.exerciseLib.GetByTags(req.ExerciseTags[0], req.ExerciseTags[1:]...)
		if err != nil {
			return err
		}
	}

	if req.Buffer == 0 {
		req.Buffer = 2
	}

	if req.Capacity == 0 {
		req.Capacity = 10
	}

	ev, err := event.New(event.Config{
		Name:     req.Name,
		Tag:      req.Tag,
		Buffer:   req.Buffer,
		Capacity: req.Capacity,
		LabConfig: lab.LabConfig{
			Frontends: req.Frontends,
			Exercises: exer,
		},
		VBoxLibrary: svc.frontendLibrary,
	})
	if err != nil {
		return err
	}

	go ev.Start(context.TODO())

	// director := func(req *http.Request) {
	// 	req.Header.Add("X-Forwarded-Host", req.Host)
	// 	req.Header.Add("X-Origin-Host", origin.Host)
	// 	req.URL.Scheme = "http"
	// 	req.URL.Host = origin.Host
	// 	fmt.Println(req.URL.Path)
	// }

	subdomain := req.Tag + ".localhost"
	eventRoute := svc.mux.Host(subdomain).Subrouter()
	ev.Connect(eventRoute)
	svc.events[req.Tag] = ev

	return nil
}

type StopEventRequest struct {
	Tag string `json:"tag"`
}

func (svc *service) StopEvent(req StopEventRequest) error {
	ev, ok := svc.events[req.Tag]
	if !ok {
		return UnknownEventErr
	}

	delete(svc.events, req.Tag)

	ev.Close()
	return nil
}

func (svc *service) Close() {
	for t, ev := range svc.events {
		ev.Close()
		delete(svc.events, t)
	}
}
