package ctfd_test

import (
	"crypto/sha256"
	"fmt"
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

			sm := ctfd.NewSessionMap()

			var ranPreHook bool
			pre := func() error { ranPreHook = true; return nil }

			var ranPostHook bool
			post := func(store.Team) error { ranPostHook = true; return nil }

			interceptor := ctfd.NewRegisterInterception(sm, []func() error{pre}, []func(store.Team) error{post})
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
			hashedFormPass := fmt.Sprintf("%x", sha256.Sum256([]byte(f.Get("password"))))
			if password != hashedFormPass {
				t.Fatalf("expected password to be hashed with sha256")
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

			_, err := sm.GetEmailBySession(session)
			if err != nil {
				t.Fatalf("expected no error when fetching team's email by session: %s", err)
			}

			if !ranPreHook {
				t.Fatalf("expected pre hook to have been run")
			}

			if !ranPostHook {
				t.Fatalf("expected post hook to have been run")
			}

		})
	}

}

func TestCheckFlagInterceptor(t *testing.T) {
	host := "http://sec02.lab.es.aau.dk"
	flag := "some_flag_here"
	knownSession := "known_session"
	email := "some@email.com"

	sm := ctfd.NewSessionMap()
	sm.SetSessionForEmail(knownSession, email)

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
		{name: "Normal", path: "/chal/1", method: "POST", form: &validForm, tagMap: map[int]store.Tag{1: "hb"}, task: &store.Task{OwnerEmail: email, ExerciseTag: "hb"}, session: knownSession, intercept: true},
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

			interceptor := ctfd.NewCheckFlagInterceptor(sm, tc.tagMap, post)
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
				if tc.task.ExerciseTag != task.ExerciseTag {
					t.Fatalf("mismatch across exercise tag (expected: %s), received: %s", tc.task.ExerciseTag, task.ExerciseTag)
				}

				if tc.task.OwnerEmail != task.OwnerEmail {
					t.Fatalf("mismatch across owner email (expected: %s), received: %s", tc.task.OwnerEmail, task.OwnerEmail)
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

			sm := ctfd.NewSessionMap()
			interceptor := ctfd.NewLoginInterceptor(sm)
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
			hashedFormPass := fmt.Sprintf("%x", sha256.Sum256([]byte(f.Get("password"))))
			if password != hashedFormPass {
				t.Fatalf("expected password to be hashed with sha256")
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

			_, err := sm.GetEmailBySession(session)
			if err != nil {
				t.Fatalf("expected no error when fetching team's email by session: %s", err)
			}
		})
	}

}
