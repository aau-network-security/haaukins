package ctfd

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"

	"github.com/aau-network-security/go-ntp/store"
	"github.com/rs/zerolog/log"
)

var (
	chalPathRegex = regexp.MustCompile(`/chal/([0-9]+)`)
)

type RegisterInterceptOpts func(*registerInterception)

func WithPreRegisterHooks(hooks ...func(*store.Team) error) RegisterInterceptOpts {
	return func(ri *registerInterception) {
		ri.preHooks = append(ri.preHooks, hooks...)
	}
}

func WithPostRegisterHooks(hooks ...func(store.Team) error) RegisterInterceptOpts {
	return func(ri *registerInterception) {
		ri.postHooks = append(ri.postHooks, hooks...)
	}
}

func NewRegisterInterception(ts store.TeamStore, opts ...RegisterInterceptOpts) *registerInterception {
	ri := &registerInterception{
		teamStore: ts,
	}

	for _, opt := range opts {
		opt(ri)
	}

	return ri
}

type registerInterception struct {
	preHooks  []func(*store.Team) error
	postHooks []func(store.Team) error
	teamStore store.TeamStore
}

func (*registerInterception) ValidRequest(r *http.Request) bool {
	if r.URL.Path == "/register" && r.Method == http.MethodPost {
		return true
	}

	return false
}

func (ri *registerInterception) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.FormValue("name")
		email := r.FormValue("email")
		pass := r.FormValue("password")
		t := store.NewTeam(email, name, pass)

		for _, h := range ri.preHooks {
			if err := h(&t); err != nil {
				w.Write([]byte(fmt.Sprintf("<h1>%s</h1>", err)))
				return
			}
		}

		r.Form.Set("password", t.HashedPassword)

		// update body and content-length
		formdata := r.Form.Encode()
		r.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(formdata)))
		r.ContentLength = int64(len(formdata))

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

			for _, h := range ri.postHooks {
				if err := h(t); err != nil {
					log.Warn().
						Err(err).
						Str("name", t.Name).
						Msg("Unable to run post hook")
				}
			}
		}

	})
}

type challengeResp struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type checkFlagInterception struct {
	teamStore store.TeamStore
	flagPool  *flagPool
	postHooks []func(store.Challenge) error
}

func NewCheckFlagInterceptor(ts store.TeamStore, fp *flagPool, post ...func(store.Challenge) error) *checkFlagInterception {
	return &checkFlagInterception{
		teamStore: ts,
		flagPool:  fp,
		postHooks: post,
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
		t, err := cfi.getTeamFromSession(r)
		if err != nil {
			log.Warn().
				Err(err).
				Msg("Unable to get team based on session")
			return
		}

		matches := chalPathRegex.FindStringSubmatch("/" + r.URL.Path)
		chalNumStr := matches[1]
		cid, _ := strconv.Atoi(chalNumStr)

		flagValue := r.FormValue("key")

		value := cfi.flagPool.TranslateFlagForTeam(t, cid, flagValue)
		r.Form.Set("key", value)

		// update body and content-length
		formdata := r.Form.Encode()
		r.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(formdata)))
		r.ContentLength = int64(len(formdata))

		resp, body := recordAndServe(next, r, w)
		defer resp.Body.Close()

		var chal challengeResp
		if err := json.Unmarshal(body, &chal); err != nil {
			log.Warn().
				Err(err).
				Msg("Unable to read response from flag intercept")
			return
		}

		if strings.ToLower(chal.Message) == "correct" {
			tag, err := cfi.flagPool.GetTagByIdentifier(cid)
			if err != nil {
				log.Warn().
					Err(err).
					Msg("Unable to find challenge tag for identifier")
				return
			}

			err = t.SolveChallenge(tag, value)
			if err != nil {
				log.Warn().
					Err(err).
					Str("team-id", t.Id).
					Msg("Unable to solve challenge for team")
				return
			}

			err = cfi.teamStore.SaveTeam(t)
			if err != nil {
				log.Warn().
					Err(err).
					Msg("Unable to save team")
				return
			}
		}

	})
}

func (cfi *checkFlagInterception) getTeamFromSession(r *http.Request) (store.Team, error) {
	c, err := r.Cookie("session")
	if err != nil {
		return store.Team{}, fmt.Errorf("Unable to find session cookie")
	}

	session := c.Value
	t, err := cfi.teamStore.GetTeamByToken(session)
	if err != nil {
		return store.Team{}, err
	}

	return t, nil
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

		// update body and content-length
		formdata := r.Form.Encode()
		r.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(formdata)))
		r.ContentLength = int64(len(formdata))

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
