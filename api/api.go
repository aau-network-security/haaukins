package api

import (
	"encoding/json"
	"fmt"
	"github.com/aau-network-security/go-ntp/event"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"net/http"
)

type Api struct {
	Event event.Event
}

func (api Api) handleRegister(w http.ResponseWriter, r *http.Request) {
	auth, err := api.Event.Register(event.Group{Name: "todo"})
	if err != nil {
		log.Warn().Msgf("%s", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	log.Debug().Msgf("Registering new group with auth %+v..", auth)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(auth)
}

func (api Api) RunServer(host string, port int) {
	router := mux.NewRouter()
	router.HandleFunc("/register", api.handleRegister).Methods("POST")
	log.Fatal().Msgf("%s", http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), router))
}
