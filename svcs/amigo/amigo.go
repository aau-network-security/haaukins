package amigo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/aau-network-security/haaukins"
	"github.com/aau-network-security/haaukins/store"
	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
)

const (
	ID_KEY = "I"
)

var (
	ErrReadBodyTooLarge   = errors.New("request body is too large")
	ErrUnauthorized       = errors.New("requires authentication")
	ErrInvalidTokenFormat = errors.New("invalid token format")
	ErrInvalidFlag        = errors.New("invalid flag")
)

type Amigo struct {
	maxReadBytes int64
	signingKey   []byte
	teamStore    store.TeamStore
}

type AmigoOpt func(*Amigo)

func WithMaxReadBytes(b int64) AmigoOpt {
	return func(am *Amigo) {
		am.maxReadBytes = b
	}
}

func NewAmigo(ts store.TeamStore, key string, opts ...AmigoOpt) *Amigo {
	am := &Amigo{
		maxReadBytes: 1024 * 1024,
		signingKey:   []byte(key),
		teamStore:    ts,
	}

	for _, opt := range opts {
		opt(am)
	}

	return am
}

func (am *Amigo) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/flags/verify", am.handleFlagVerify())
	return m
}

func (am *Amigo) handleFlagVerify() http.HandlerFunc {
	type verifyFlagMsg struct {
		Flag string `json:"flag"`
	}

	type replyMsg struct {
		Status string `json:"status"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		team, err := am.getTeamFromRequest(w, r)
		if err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		var msg verifyFlagMsg
		if err := safeReadJson(w, r, &msg, am.maxReadBytes); err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		uid, err := uuid.Parse(msg.Flag)
		if err != nil {
			replyJson(http.StatusOK, w, errReply{ErrInvalidFlag.Error()})
			return
		}
		flag := haaukins.Flag(uid)

		if err := team.VerifyFlag(flag); err != nil {
			replyJson(http.StatusOK, w, errReply{err.Error()})
			return
		}

		replyJson(http.StatusOK, w, replyMsg{"ok"})
	}
}

func (am *Amigo) getTeamFromRequest(w http.ResponseWriter, r *http.Request) (*haaukins.Team, error) {
	c, err := r.Cookie("session")
	if err != nil {
		return nil, ErrUnauthorized
	}
	token := c.Value

	replyErr := func(err error) (*haaukins.Team, error) {
		http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: 0})
		return nil, err
	}

	jwtToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return am.signingKey, nil
	})
	if err != nil {
		return replyErr(err)
	}

	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok || !jwtToken.Valid {
		return replyErr(ErrInvalidTokenFormat)
	}

	id, ok := claims[ID_KEY].(string)
	if !ok {
		return replyErr(ErrInvalidTokenFormat)
	}

	team, err := am.teamStore.GetTeamByID(id)
	if err != nil {
		return replyErr(err)
	}

	return team, nil
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

func replyJsonRequestErr(w http.ResponseWriter, err error) error {
	return replyJson(http.StatusBadRequest, w, errReply{err.Error()})
}

func GetTokenForTeam(key []byte, t *haaukins.Team) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		ID_KEY: t.Id,
	})

	tokenStr, err := token.SignedString(key)
	if err != nil {
		return "", err
	}

	return tokenStr, nil
}
