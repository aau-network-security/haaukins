package event

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/aau-network-security/go-ntp/virtual"
	"github.com/aau-network-security/go-ntp/virtual/docker"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

type callbackServer struct {
	event Event
	srv   *http.Server
	host  string
	port  uint
}

type RegisterRequest struct {
	Name string `json:"name"`
}

type RegisterResponse struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (cb *callbackServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	var input RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeErr(w, err)
		return
	}

	auth, err := cb.event.Register(Group{Name: input.Name})
	if err != nil {
		writeErr(w, err)
		return
	}

	resp := RegisterResponse{
		Username: auth.Username,
		Password: auth.Password,
	}

	writeReply(w, &resp, http.StatusOK)
}

func (cb *callbackServer) Run() error {
	host, err := docker.GetDockerHostIP()
	if err != nil {
		return err
	}
	cb.host = host
	cb.port = virtual.GetAvailablePort()

	addr := fmt.Sprintf("%s:%d", cb.host, cb.port)

	router := mux.NewRouter()
	router.HandleFunc("/register", cb.handleRegister).Methods("POST")

	h := &http.Server{Addr: addr, Handler: router}
	go func() {
		if err := h.ListenAndServe(); err != nil {
			log.Fatal().Err(err).Msg("error running callback server for event")
		}
	}()

	cb.srv = h
	return nil
}

func (cb *callbackServer) Close() {
	if cb.srv == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cb.srv.Shutdown(ctx)
}

type ErrorResponse struct {
	Msg string `json:"error"`
}

func writeErr(w http.ResponseWriter, err error) {
	writeReply(w, &ErrorResponse{Msg: err.Error()}, http.StatusBadRequest)
}

func writeReply(w http.ResponseWriter, i interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")

	b, err := json.Marshal(i)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{ "error":"Internal Server Error"}`))

		log.Warn().
			Str("error", err.Error()).
			Msg("Error writing json response")
	}

	w.WriteHeader(status)
	w.Write(b)
}
