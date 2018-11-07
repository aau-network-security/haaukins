package ctfd_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/svcs/ctfd"
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

	tt := []struct {
		name      string
		path      string
		method    string
		sendFlag  string
		value     string
		flag      *store.FlagConfig
		solve     bool
		intercept bool
	}{
		{name: "Static (incorrect)", path: "/chal/1", method: "POST", sendFlag: "incorrect", value: "abc", flag: &store.FlagConfig{Tag: "tst", Static: "abcde"}, intercept: true},
		{name: "Static (correct)", path: "/chal/1", method: "POST", sendFlag: "abc", value: "abc", flag: &store.FlagConfig{Tag: "tst", Static: "abcde"}, solve: true, intercept: true},
		{name: "Dynamic (incorrect)", path: "/chal/1", method: "POST", sendFlag: "incorrect", value: "abc", flag: &store.FlagConfig{Tag: "tst", EnvVar: "flag"}, intercept: true},
		{name: "Dynamic (correct)", path: "/chal/1", method: "POST", sendFlag: "abc", value: "abc", flag: &store.FlagConfig{Tag: "tst", EnvVar: "flag"}, solve: true, intercept: true},
		{name: "Index", path: "/", method: "GET", intercept: false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, host+tc.path, nil)
			if tc.sendFlag != "" {
				f := url.Values{
					"key":   {tc.sendFlag},
					"nonce": {nonce},
				}
				req = httptest.NewRequest(tc.method, host+tc.path, strings.NewReader(f.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			}

			ts := store.NewTeamStore()
			fp := ctfd.NewFlagPool()

			var ctfdValue string
			if tc.flag != nil {
				ctfdValue = fp.AddFlag(*tc.flag, 1)

				team := store.NewTeam(email, "name_goes_here", "passhere", store.Challenge{FlagTag: tc.flag.Tag, FlagValue: tc.value})
				if err := ts.CreateTeam(team); err != nil {
					t.Fatalf("expected to be able to create team")
				}

				if err := ts.CreateTokenForTeam(knownSession, team); err != nil {
					t.Fatalf("expected to be able to create token for team")
				}

				cookie := http.Cookie{Name: "session", Value: knownSession}
				req.AddCookie(&cookie)
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
			output := `{"message":"Correct", "status": 1}`
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key = r.FormValue("key")
				readNonce = r.FormValue("nonce")

				w.Write([]byte(output))
				return
			})

			if !tc.intercept {
				t.Fatalf("intercepted despite not correct request")
			}

			w := httptest.NewRecorder()
			interceptor.Intercept(testHandler).ServeHTTP(w, req)

			content, err := ioutil.ReadAll(w.Result().Body)
			if err != nil {
				t.Fatalf("unable to read response body")
			}

			if string(content) != output {
				t.Fatalf("response does not match expectation")
			}

			if readNonce != nonce {
				t.Fatalf("expected nonce (value: %s) to be parsed on, but received: %s", nonce, readNonce)
			}

			team, _ := ts.GetTeamByEmail(email)
			chal := team.ChalMap[tc.flag.Tag]

			inSolvedChallenges := false
			for _, c := range team.SolvedChallenges {
				if c.FlagTag == tc.flag.Tag {
					inSolvedChallenges = true
					break
				}
			}

			if inSolvedChallenges != tc.solve {
				t.Fatalf("unexpected appearence/missing of challenge in solved challenges")
			}

			if !tc.solve {
				if chal.CompletedAt != nil {
					t.Fatalf("expected no completion of challenge")
				}

				return
			}

			if key != ctfdValue {
				t.Fatalf("expect key to pass through interception")
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
