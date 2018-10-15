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

			ts := store.NewTeamStore()
			var ranPreHook bool
			pre := func() error { ranPreHook = true; return nil }

			interceptor := ctfd.NewRegisterInterception(ts, []func() error{pre}, []func(store.Team) error{})
			ok := interceptor.ValidRequest(req)
			if !ok {
				if tc.intercept {
					t.Fatalf("no interception, despite expected intercept")
				}

				return
			}

			var name, email, password, nonce string
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				name = r.FormValue("name")
				email = r.FormValue("email")
				password = r.FormValue("password")
				nonce = r.FormValue("nonce")

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
	flag := "some_flag_here"
	knownSession := "known_session"
	email := "some@email.com"

	ts := store.NewTeamStore()
	team := store.NewTeam(email, "name_goes_here", "passhere")
	if err := ts.CreateTeam(team); err != nil {
		t.Fatalf("expected to be able to create team")
	}

	if err := ts.CreateTokenForTeam(knownSession, team); err != nil {
		t.Fatalf("expected to be able to create token for team")
	}

	validForm := url.Values{
		"key":   {flag},
		"nonce": {"random_string"},
	}

	tt := []struct {
		name      string
		path      string
		method    string
		form      *url.Values
		tagMap    map[int]store.Tag
		session   string
		task      *store.Task
		intercept bool
	}{
		{name: "Normal", path: "/chal/1", method: "POST", form: &validForm, tagMap: map[int]store.Tag{1: "hb"}, task: &store.Task{OwnerID: team.Id, FlagTag: "hb"}, session: knownSession, intercept: true},
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

			if tc.session != "" {
				cookie := http.Cookie{Name: "session", Value: tc.session}
				req.AddCookie(&cookie)
			}

			var task *store.Task
			post := func(t store.Task) error { task = &t; return nil }

			interceptor := ctfd.NewCheckFlagInterceptor(ts, tc.tagMap, post)
			ok := interceptor.ValidRequest(req)
			if !ok {
				if tc.intercept {
					t.Fatalf("no interception, despite expected intercept")
				}

				return
			}

			var key string
			output := `{"message":"Correct", "status": 1}`
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key = r.FormValue("key")
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

			f := *tc.form
			if key != f.Get("key") {
				t.Fatalf("expect key to pass through interception")
			}

			if task == nil {
				t.Fatalf("expected post hook to have been run")
			}

			if task.CompletedAt == nil {
				t.Fatalf("expected that completion date of the exercise has been added")
			}

			if tc.task != nil {
				if tc.task.FlagTag != task.FlagTag {
					t.Fatalf("mismatch across exercise tag (expected: %s), received: %s", tc.task.FlagTag, task.FlagTag)
				}

				if tc.task.OwnerID != task.OwnerID {
					t.Fatalf("mismatch across owner id (expected: %s), received: %s", tc.task.OwnerID, task.OwnerID)
				}
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

			interceptor := ctfd.NewLoginInterceptor(ts)
			ok := interceptor.ValidRequest(req)
			if !ok {
				if tc.intercept {
					t.Fatalf("no interception, despite expected intercept")
				}

				return
			}

			var name, password, nonce string
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				name = r.FormValue("name")
				password = r.FormValue("password")
				nonce = r.FormValue("nonce")

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
