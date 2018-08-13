package api

import (
	"encoding/json"
	"fmt"
	"github.com/aau-network-security/go-ntp/event"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

type Api struct {
	Event event.Event
}

func (api Api) handleRegister(w http.ResponseWriter, r *http.Request) {
	auth, err := api.Event.Register(event.Group{Name: "todo"})
	if err != nil {
		fmt.Println(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(auth)
}

func (api Api) RunServer(host string, port int) {
	router := mux.NewRouter()
	router.HandleFunc("/register", api.handleRegister).Methods("POST")
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), router))
}
