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
	"sync"
	"time"

	"github.com/aau-network-security/go-ntp/store"
	"github.com/rs/zerolog/log"
)

var (
	chalPathRegex = regexp.MustCompile(`/chal/([0-9]+)`)
)

type sessionMap struct {
	m           sync.RWMutex
	sessToEmail map[string]string
}

func NewSessionMap() *sessionMap {
	return &sessionMap{
		sessToEmail: map[string]string{},
	}
}

func (sm *sessionMap) GetEmailBySession(s string) (string, error) {
	sm.m.RLock()
	defer sm.m.RUnlock()

	email, ok := sm.sessToEmail[s]
	if !ok {
		return "", NoSessionErr
	}

	return email, nil
}

func (sm *sessionMap) SetSessionForEmail(s, email string) {
	sm.m.Lock()
	defer sm.m.Unlock()

	sm.sessToEmail[s] = email
}

type Interception interface {
	ValidRequest(func(r *http.Request)) bool
	Intercept(http.Handler) http.Handler
}

type Interceptors []Interception

func (i Interceptors) Intercept(http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	})
}

func NewRegisterInterception(sessMap *sessionMap, pre []func() error, post []func(store.Team) error, tasks ...store.Task) *registerInterception {
	return &registerInterception{
		defaultTasks: tasks,
		preHooks:     pre,
		postHooks:    post,
		sessionMap:   sessMap,
	}
}

type registerInterception struct {
	m            sync.RWMutex
	defaultTasks []store.Task
	preHooks     []func() error
	postHooks    []func(store.Team) error
	sessionMap   *sessionMap
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
		hashedPass := fmt.Sprintf("%x", sha256.Sum256([]byte(pass)))

		r.Form.Set("password", hashedPass)

		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		for k, v := range rec.HeaderMap {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		rec.Body.WriteTo(w)

		resp := rec.Result()
		var session string
		for _, c := range resp.Cookies() {
			if c.Name == "session" {
				session = c.Value
				break
			}
		}

		if session != "" {
			t, err := store.NewTeam(email, name, hashedPass, ri.defaultTasks...)
			if err != nil {
				log.Warn().
					Err(err).
					Str("email", email).
					Str("name", name).
					Str("hashed_pass", hashedPass).
					Msg("Unable to create new team")
				return
			}

			for _, h := range ri.postHooks {
				if err := h(t); err != nil {
					log.Warn().
						Err(err).
						Str("email", email).
						Str("name", name).
						Str("hashed_pass", hashedPass).
						Msg("Post hook failed (register intercept)")
				}
			}

			ri.sessionMap.SetSessionForEmail(session, email)
		}

	})
}

type challengeResp struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type checkFlagInterception struct {
	sessionMap   *sessionMap
	challengeMap map[int]store.Tag
	postHooks    []func(store.Task) error
}

func NewCheckFlagInterceptor(sessMap *sessionMap, chalMap map[int]store.Tag, post ...func(store.Task) error) *checkFlagInterception {
	return &checkFlagInterception{
		sessionMap:   sessMap,
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
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		for k, v := range rec.HeaderMap {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)

		var rawBody bytes.Buffer
		multi := io.MultiWriter(w, &rawBody)
		rec.Body.WriteTo(multi)

		cfi.inspectResponse(r, rawBody.Bytes(), rec.Result())
	})
}

func (cfi *checkFlagInterception) inspectResponse(r *http.Request, body []byte, resp *http.Response) {
	defer resp.Body.Close()

	email, err := cfi.getEmailFromSession(r)
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
			OwnerEmail:  email,
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

func (cfi *checkFlagInterception) getEmailFromSession(r *http.Request) (string, error) {
	c, err := r.Cookie("session")
	if err != nil {
		return "", fmt.Errorf("Unable to find session cookie")
	}
	session := c.Value

	return cfi.sessionMap.GetEmailBySession(session)
}
