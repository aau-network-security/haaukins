package ctfd_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
	"github.com/google/uuid"
)

func TestRegisterInterception(t *testing.T) {
	host := "http://sec02.lab.es.aau.dk"
	validForm := url.Values{
		"email":    {"some@email.dk"},
		"name":     {"some_username"},
		"password": {"secret_password"},
		"nonce":    {"random_string"},
	}

	tt := []struct {
		name      string
		path      string
		method    string
		form      *url.Values
		intercept bool
	}{
		{name: "Normal", path: "/register", method: "POST", form: &validForm, intercept: true},
		{name: "Index", path: "/", method: "GET", intercept: false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, host+tc.path, nil)
			if tc.form != nil {
				f := *tc.form
				req = httptest.NewRequest(tc.method, host+tc.path, strings.NewReader(f.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			}
			cl := req.ContentLength

			ts := store.NewTeamStore()
			var ranPreHook bool
			pre := func(*store.Team) error { ranPreHook = true; return nil }

			interceptor := ctfd.NewRegisterInterception(ts, ctfd.WithRegisterHooks(pre))
			ok := interceptor.ValidRequest(req)
			if !ok {
				if tc.intercept {
					t.Fatalf("no interception, despite expected intercept")
				}

				return
			}

			var name, email, password, nonce string
			var postCl int64
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				name = r.FormValue("name")
				email = r.FormValue("email")
				password = r.FormValue("password")
				nonce = r.FormValue("nonce")

				postCl = req.ContentLength

				expiration := time.Now().Add(365 * 24 * time.Hour)
				cookie := http.Cookie{Name: "session", Value: "secret-cookie", Expires: expiration}
				http.SetCookie(w, &cookie)

				return
			})

			if !tc.intercept {
				t.Fatalf("intercepted despite not correct request")
			}

			w := httptest.NewRecorder()
			interceptor.Intercept(testHandler).ServeHTTP(w, req)

			f := *tc.form
			orgPassword := f.Get("password")
			if password == orgPassword {
				t.Fatalf("expected password to be changed (org: %s), received: %s", orgPassword, password)
			}

			if cl == postCl {
				t.Fatalf("expected content-length (pre: %d) to change after interception, received: %d", cl, postCl)
			}

			if f.Get("name") != name {
				t.Fatalf("expected name to be untouched")
			}

			if f.Get("nonce") != nonce {
				t.Fatalf("expected nonce to be untouched")
			}

			if f.Get("email") != email {
				t.Fatalf("expected email to be untouched")
			}

			resp := w.Result()
			var session string
			for _, c := range resp.Cookies() {
				if c.Name == "session" {
					session = c.Value
				}
			}

			if session == "" {
				t.Fatalf("expected session to be none empty")
			}

			_, err := ts.GetTeamByToken(session)
			if err != nil {
				t.Fatalf("expected no error when fetching team's email by session: %s", err)
			}

			if !ranPreHook {
				t.Fatalf("expected pre hook to have been run")
			}
		})
	}

}

func TestCheckFlagInterceptor(t *testing.T) {
	host := "http://sec02.lab.es.aau.dk"
	knownSession := "known_session"
	email := "some@email.com"
	nonce := "some_nonce"

	genFlags := func(value string, static bool, n int) (*ctfd.FlagPool, store.Tag, int, string) {
		fp := ctfd.NewFlagPool()
		var flags []store.FlagConfig
		for i := 0; i < n-1; i++ {
			flag := store.FlagConfig{
				Tag:    store.Tag(uuid.New().String()),
				EnvVar: uuid.New().String(),
			}

			if n := rand.Intn(1); n > 0 {
				flag.Static = uuid.New().String()
			}

			flags = append(flags, flag)
		}

		flagtag := store.Tag(uuid.New().String())
		flag := store.FlagConfig{
			Tag:    flagtag,
			EnvVar: "tst",
		}
		if static {
			flag.Static = value
		}

		flags = append(flags, flag)

		rand.Seed(time.Now().UnixNano())
		perm := rand.Perm(len(flags))

		// shuffle
		var ctfdValue string
		for i, v := range perm {
			flag := flags[i]
			ctfval := fp.AddFlag(flag, v)
			if flag.Tag == flagtag {
				ctfdValue = ctfval
			}
		}

		id, _ := fp.GetIdentifierByTag(flagtag)

		return fp, flagtag, id, ctfdValue
	}

	tt := []struct {
		name      string
		sendFlag  string
		flagValue string
		static    bool
		solve     bool
		intercept bool
	}{
		{name: "Static (incorrect)", sendFlag: "incorrect", flagValue: "abc", intercept: true},
		{name: "Static (correct)", sendFlag: "abc", flagValue: "abc", static: true, solve: true, intercept: true},
		{name: "Dynamic (incorrect)", sendFlag: "incorrect", flagValue: "abc", intercept: true},
		{name: "Dynamic (correct)", sendFlag: "abc", flagValue: "abc", solve: true, intercept: true},
		{name: "No flags", sendFlag: "abc", intercept: true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			fp, flagtag, id, ctfdValue := genFlags(tc.flagValue, tc.static, 50)
			ts := store.NewTeamStore()

			team := store.NewTeam(email, "name_goes_here", "passhere")
			if err := ts.CreateTeam(team); err != nil {
				t.Fatalf("expected to be able to create team")
			}

			if err := ts.CreateTokenForTeam(knownSession, team); err != nil {
				t.Fatalf("expected to be able to create token for team")
			}

			f := url.Values{
				"key":   {tc.sendFlag},
				"nonce": {nonce},
			}
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("%s%s%d", host, "/chal/", id), strings.NewReader(f.Encode()))
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: "session", Value: knownSession})

			if tc.flagValue != "" {
				team.AddChallenge(store.Challenge{FlagTag: flagtag, FlagValue: tc.flagValue})
				ts.SaveTeam(team)
			}

			interceptor := ctfd.NewCheckFlagInterceptor(ts, fp)
			ok := interceptor.ValidRequest(req)
			if !ok {
				if tc.intercept {
					t.Fatalf("no interception, despite expected intercept")
				}

				return
			}

			var key string
			var readNonce string
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key = r.FormValue("key")
				readNonce = r.FormValue("nonce")

				if key == ctfdValue {
					w.Write([]byte(`{"message":"Correct", "status": 1}`))
					return
				}

				w.Write([]byte(`{"message":"Incorrect", "status": 0}`))
			})

			if !tc.intercept {
				t.Fatalf("intercepted despite not correct request")
			}

			w := httptest.NewRecorder()
			interceptor.Intercept(testHandler).ServeHTTP(w, req)

			var respJson struct {
				M string `json:"message"`
				S int    `json:"status"`
			}
			err := json.NewDecoder(w.Result().Body).Decode(&respJson)
			if err != nil {
				t.Fatalf("unable to read json response body")
			}

			if readNonce != nonce {
				t.Fatalf("expected nonce (value: %s) to be parsed on, but received: %s", nonce, readNonce)
			}

			team, _ = ts.GetTeamByEmail(email)
			chal := team.ChalMap[flagtag]

			inSolvedChallenges := false
			for _, c := range team.SolvedChallenges {
				if c.FlagTag == flagtag {
					inSolvedChallenges = true
					break
				}
			}

			if inSolvedChallenges != tc.solve {
				t.Fatalf("missing challenge in solved challenges for team")
			}

			if !tc.solve {
				if chal.CompletedAt != nil {
					t.Fatalf("expected no completion of challenge")
				}

				return
			}

			if chal.CompletedAt == nil {
				t.Fatalf("expected that completion date of the exercise has been added")
			}
		})
	}

}

func TestLoginInterception(t *testing.T) {
	host := "http://sec02.lab.es.aau.dk"
	knownEmail := "some@email.dk"
	validForm := url.Values{
		"name":     {knownEmail},
		"password": {"secret_password"},
		"nonce":    {"random_string"},
	}

	ts := store.NewTeamStore()
	team := store.NewTeam(knownEmail, "name_goes_here", "passhere")
	if err := ts.CreateTeam(team); err != nil {
		t.Fatalf("expected to be able to create team")
	}

	tt := []struct {
		name      string
		path      string
		method    string
		form      *url.Values
		intercept bool
	}{
		{name: "Normal", path: "/login", method: "POST", form: &validForm, intercept: true},
		{name: "Index", path: "/", method: "GET", intercept: false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, host+tc.path, nil)
			if tc.form != nil {
				f := *tc.form
				req = httptest.NewRequest(tc.method, host+tc.path, strings.NewReader(f.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			}
			cl := req.ContentLength

			interceptor := ctfd.NewLoginInterceptor(ts)
			ok := interceptor.ValidRequest(req)
			if !ok {
				if tc.intercept {
					t.Fatalf("no interception, despite expected intercept")
				}

				return
			}

			var name, password, nonce string
			var postCl int64
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				name = r.FormValue("name")
				password = r.FormValue("password")
				nonce = r.FormValue("nonce")

				postCl = r.ContentLength

				expiration := time.Now().Add(365 * 24 * time.Hour)
				cookie := http.Cookie{Name: "session", Value: "secret-cookie", Expires: expiration}
				http.SetCookie(w, &cookie)

				return
			})

			if !tc.intercept {
				t.Fatalf("intercepted despite not correct request")
			}

			w := httptest.NewRecorder()
			interceptor.Intercept(testHandler).ServeHTTP(w, req)

			f := *tc.form
			orgPassword := f.Get("password")
			if password == orgPassword {
				t.Fatalf("expected password to be changed")
			}

			if cl == postCl {
				t.Fatalf("expected content-length (pre: %d) to change after interception, received: %d", cl, postCl)
			}

			if f.Get("name") != name {
				t.Fatalf("expected name to be untouched")
			}

			if f.Get("nonce") != nonce {
				t.Fatalf("expected nonce to be untouched")
			}

			resp := w.Result()
			var session string
			for _, c := range resp.Cookies() {
				if c.Name == "session" {
					session = c.Value
				}

			}

			if session == "" {
				t.Fatalf("expected session to be none empty")
			}

			_, err := ts.GetTeamByToken(session)
			if err != nil {
				t.Fatalf("expected no error when fetching team by session: %s", err)
			}
		})
	}

}
