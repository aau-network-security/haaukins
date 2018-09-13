package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

const (
	MaxFileSize = 1 * 1024 * 1024 // 1 MB
)

type server struct {
	svc Service
}

func NewServer() (*server, error) {
	svc, err := NewService()
	if err != nil {
		return nil, err
	}

	return &server{
		svc: svc,
	}, nil
}

func (s *server) handleCreateEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateEventRequest

		err := readJSON(w, r, &req)
		if err != nil {
			return
		}

		err = s.svc.CreateEvent(req)
		if err != nil {
			replyError(w, err, http.StatusBadRequest)
			return
		}

		replyOutput(w, "ok")
	}
}

func (s *server) handleStopEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req StopEventRequest

		err := readJSON(w, r, &req)
		if err != nil {
			return
		}

		err = s.svc.StopEvent(req)
		if err != nil {
			replyError(w, err, http.StatusBadRequest)
			return
		}

		replyOutput(w, "ok")
	}
}

func (s *server) Routes() http.Handler {
	m := mux.NewRouter()
	m.HandleFunc("/events", s.handleCreateEvent()).Methods("POST")
	m.HandleFunc("/events", s.handleStopEvent()).Methods("DELETE")

	return m
}

type Reply struct {
	Error  string      `json:"error,omitempty"`
	Output interface{} `json:"output,omitempty"`
}

func readJSON(w http.ResponseWriter, r *http.Request, i interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, MaxFileSize)
	if err := json.NewDecoder(r.Body).Decode(i); err != nil {
		replyError(w, err, http.StatusBadRequest)
		return err
	}

	return nil
}

func writeJSON(w http.ResponseWriter, i interface{}, status int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(i); err != nil {
		return err
	}

	return nil
}

func replyOutput(w http.ResponseWriter, i interface{}) error {
	return writeJSON(w, Reply{Output: i}, http.StatusOK)
}

func replyError(w http.ResponseWriter, err error, status int) error {
	return writeJSON(w, Reply{Error: err.Error()}, status)
}
