package ctfd_test

import (
	"crypto/sha256"
	"fmt"
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

			interceptor := ctfd.NewRegisterInterception()
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
			interceptor.Close()

			if session == "" {
				t.Fatalf("expected session to be none empty")
			}

			_, err := interceptor.GetTeamEmailBySession(session)
			if err != nil {
				t.Fatalf("expected no error when fetching team's email by session: %s", err)
			}

			var n int
			var te store.Team
			for team := range interceptor.TeamStream() {
				te = team
				n += 1
			}

			if n != 1 {
				t.Fatalf("expected receive one team only, got: %d", n)
			}

			if te.Email == "" {
				t.Fatalf("expected to receive a team")
			}

		})
	}

}
