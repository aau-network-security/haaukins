package api

import (
	"encoding/json"
	"fmt"
	"github.com/aau-network-security/go-ntp/event"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strings"
)

type Api struct {
	Event event.Event
}

type Auth struct {
	Username string
	Password string
}

func NewAuth() Auth {
	return Auth{
		Username: rand(),
		Password: rand()}
}

func rand() string {
	return strings.Replace(fmt.Sprintf("%v", uuid.New()), "-", "", -1)
}

func (api Api) handleRegister(w http.ResponseWriter, r *http.Request) {
	auth := NewAuth()
	api.Event.Guac.CreateUser(auth.Username, auth.Password)
	json.NewEncoder(w).Encode(auth)
}

func (api Api) RunServer(host string, port int) {
	router := mux.NewRouter()
	router.HandleFunc("/register", api.handleRegister).Methods("POST")
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), router))
}
