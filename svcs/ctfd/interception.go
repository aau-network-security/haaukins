package ctfd

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aau-network-security/go-ntp/store"
	"github.com/rs/zerolog/log"
)

var (
	chalPathRegex = regexp.MustCompile(`/chal/([0-9]+)`)
)

type Interception interface {
	ValidRequest(func(r *http.Request)) bool
	Intercept(http.Handler) http.Handler
}

type Interceptors []Interception

func (i Interceptors) Intercept(http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	})
}

func NewRegisterInterception(ts store.TeamStore, pre []func() error, tasks ...store.Task) *registerInterception {
	return &registerInterception{
		defaultTasks: tasks,
		preHooks:     pre,
		teamStore:    ts,
	}
}

type registerInterception struct {
	defaultTasks []store.Task
	preHooks     []func() error
	teamStore    store.TeamStore
}

func (*registerInterception) ValidRequest(r *http.Request) bool {
	if r.URL.Path == "/register" && r.Method == http.MethodPost {
		return true
	}

	return false
}

func (ri *registerInterception) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range ri.preHooks {
			if err := h(); err != nil {
				w.Write([]byte(fmt.Sprintf("<h1>%s</h1>", err)))
				return
			}
		}

		name := r.FormValue("name")
		email := r.FormValue("email")
		pass := r.FormValue("password")
		t, err := store.NewTeam(email, name, pass, ri.defaultTasks...)
		if err != nil {
			log.Warn().
				Err(err).
				Str("email", email).
				Str("name", name).
				Msg("Unable to create new team")
			return
		}

		r.Form.Set("password", t.HashedPassword)
		resp, _ := recordAndServe(next, r, w)

		var session string
		for _, c := range resp.Cookies() {
			if c.Name == "session" {
				session = c.Value
				break
			}
		}

		if session != "" {
			if err := ri.teamStore.CreateTeam(t); err != nil {
				log.Warn().
					Err(err).
					Str("email", t.Email).
					Str("name", t.Name).
					Msg("Unable to store new team")
				return
			}

			if err := ri.teamStore.CreateTokenForTeam(session, t); err != nil {
				log.Warn().
					Err(err).
					Str("email", t.Email).
					Str("name", t.Name).
					Msg("Unable to store session token for team")
				return
			}
		}

	})
}

type challengeResp struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type checkFlagInterception struct {
	teamStore    store.TeamStore
	challengeMap map[int]store.Tag
	postHooks    []func(store.Task) error
}

func NewCheckFlagInterceptor(ts store.TeamStore, chalMap map[int]store.Tag, post ...func(store.Task) error) *checkFlagInterception {
	return &checkFlagInterception{
		teamStore:    ts,
		challengeMap: chalMap,
		postHooks:    post,
	}
}

func (*checkFlagInterception) ValidRequest(r *http.Request) bool {
	if r.Method == http.MethodPost && chalPathRegex.MatchString(r.URL.Path) {
		return true
	}

	return false
}

func (cfi *checkFlagInterception) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, body := recordAndServe(next, r, w)

		cfi.inspectResponse(r, body, resp)
	})
}

func (cfi *checkFlagInterception) inspectResponse(r *http.Request, body []byte, resp *http.Response) {
	defer resp.Body.Close()

	id, err := cfi.getIDFromSession(r)
	if err != nil {
		log.Warn().
			Err(err).
			Msg("Unable to get team based on session")
		return
	}

	var chal challengeResp
	if err := json.Unmarshal(body, &chal); err != nil {
		log.Warn().
			Err(err).
			Msg("Unable to read response from flag intercept")
		return
	}

	matches := chalPathRegex.FindStringSubmatch(r.URL.Path)
	chalNumStr := matches[1]
	chalNum, _ := strconv.Atoi(chalNumStr)

	tag, ok := cfi.challengeMap[chalNum]
	if !ok {
		log.Warn().
			Int("chal_id", chalNum).
			Msg("Unknown challenge by id")
		return
	}

	if strings.ToLower(chal.Message) == "correct" {
		now := time.Now()
		task := store.Task{
			OwnerID:     id,
			ExerciseTag: tag,
			CompletedAt: &now,
		}

		for _, h := range cfi.postHooks {
			if err := h(task); err != nil {
				log.Warn().
					Int("chal_id", chalNum).
					Msg("Unknown challenge by id")
				return
			}
		}
	}
}

func (cfi *checkFlagInterception) getIDFromSession(r *http.Request) (string, error) {
	c, err := r.Cookie("session")
	if err != nil {
		return "", fmt.Errorf("Unable to find session cookie")
	}
	session := c.Value
	t, err := cfi.teamStore.GetTeamByToken(session)
	if err != nil {
		return "", err
	}

	return t.Id, nil
}

type loginInterception struct {
	teamStore store.TeamStore
}

func NewLoginInterceptor(ts store.TeamStore) *loginInterception {
	return &loginInterception{
		teamStore: ts,
	}
}

func (*loginInterception) ValidRequest(r *http.Request) bool {
	if r.URL.Path == "/login" && r.Method == http.MethodPost {
		return true
	}

	return false
}

func (li *loginInterception) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.FormValue("name")
		pass := r.FormValue("password")
		hashedPass := fmt.Sprintf("%x", sha256.Sum256([]byte(pass)))
		r.Form.Set("password", hashedPass)

		resp, _ := recordAndServe(next, r, w)

		var session string
		for _, c := range resp.Cookies() {
			if c.Name == "session" {
				session = c.Value
				break
			}
		}

		var t store.Team
		var err error
		t, err = li.teamStore.GetTeamByEmail(name)
		if err != nil {
			t, err = li.teamStore.GetTeamByName(name)
		}

		if err != nil {
			log.Warn().
				Str("name", name).
				Msg("Unknown team with name/email")
			return
		}

		if session != "" {
			li.teamStore.CreateTokenForTeam(session, t)
		}
	})
}

func recordAndServe(next http.Handler, r *http.Request, w http.ResponseWriter) (*http.Response, []byte) {
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, r)
	for k, v := range rec.HeaderMap {
		w.Header()[k] = v
	}
	w.WriteHeader(rec.Code)

	var rawBody bytes.Buffer
	multi := io.MultiWriter(w, &rawBody)
	rec.Body.WriteTo(multi)

	return rec.Result(), rawBody.Bytes()
}
