package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aau-network-security/go-ntp/event"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

type Api struct {
	Event event.Event
}

type RegisterRequest struct {
	Name string `json:"name"`
}

type RegisterResponse struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (api Api) handleRegister(w http.ResponseWriter, r *http.Request) {
	var input RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeErr(w, err)
		return
	}

	auth, err := api.Event.Register(event.Group{Name: input.Name})
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

func (api Api) RunServer(host string, port int) {
	router := mux.NewRouter()
	router.HandleFunc("/register", api.handleRegister).Methods("POST")
	log.Fatal().Msgf("%s", http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), router))
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
		w.Write([]byte(`{ "error":"Internal Server Error"}`))
		w.WriteHeader(http.StatusInternalServerError)

		log.Warn().
			Str("error", err.Error()).
			Msg("Error writing json response")
	}

	w.WriteHeader(status)
	w.Write(b)
}
