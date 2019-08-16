package amigo

import (
	"encoding/json"
	"errors"
	"net/http"
)

var (
	ErrReadBodyTooLarge = errors.New("request body is too large")
)

type Amigo struct {
	maxReadBytes int64
}

type AmigoOpt func(*Amigo)

func WithMaxReadBytes(b int64) AmigoOpt {
	return func(am *Amigo) {
		am.maxReadBytes = b
	}
}

func NewAmigo(opts ...AmigoOpt) *Amigo {

	am := &Amigo{
		maxReadBytes: 1024 * 1024,
	}

	for _, opt := range opts {
		opt(am)
	}

	return am
}

func (am *Amigo) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/flags/verify", am.handleFlag())
	return m
}

func (am *Amigo) handleFlag() http.HandlerFunc {
	type verifyFlagMsg struct {
		Flag string `json:"flag"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var msg verifyFlagMsg

		if err := safeReadJson(w, r, &msg, am.maxReadBytes); err != nil {
			replyJson(http.StatusBadRequest, w, errReply{err.Error()})
		}
	}
}

func safeReadJson(w http.ResponseWriter, r *http.Request, i interface{}, bytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, bytes)
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(i); err != nil {
		switch err.Error() {
		case "http: request body too large":
			return ErrReadBodyTooLarge
		default:
			return err
		}
	}

	return nil
}

func replyJson(sc int, w http.ResponseWriter, i interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(sc)

	return json.NewEncoder(w).Encode(i)
}

type errReply struct {
	Error string `json:"error"`
}
