package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/aau-network-security/go-ntp/event"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
)

type service struct {
	Event event.Event
}

type Service interface {
	CreateEvent() error
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
	router.Use(logging)

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
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{ "error":"Internal Server Error"}`))

		log.Warn().
			Str("error", err.Error()).
			Msg("Error writing json response")
	}

	w.WriteHeader(status)
	w.Write(b)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		log.Debug().
			Str("path", r.RequestURI).
			Str("response", fmt.Sprintf("%q", rec.Body)).
			Msg("HTTP Request")

		for k, v := range rec.HeaderMap {
			w.Header()[k] = v
		}

		w.WriteHeader(rec.Code)

		b := rec.Body.Bytes()
		w.Write(b)
	})
}
