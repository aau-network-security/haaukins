package guacamole_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aau-network-security/go-ntp/store"
	"github.com/aau-network-security/go-ntp/svcs/guacamole"
)

func TestGuacLoginTokenInterceptor(t *testing.T) {
	host := "http://sec02.lab.es.aau.dk"
	knownSession := "some_known_session"

	ts := store.NewTeamStore()
	team, _ := store.NewTeam("email@here.com", "name_goes_here", "passhere")
	if err := ts.CreateTeam(team); err != nil {
		t.Fatalf("expected to be able to create team")
	}

	if err := ts.CreateTokenForTeam(knownSession, team); err != nil {
		t.Fatalf("expected to be able to create token for team")
	}

	us := guacamole.NewGuacUserStore()
	us.CreateUserForTeam(team.Id, guacamole.GuacUser{Username: "some-user", Password: "some-pass"})

	tt := []struct {
		name      string
		path      string
		method    string
		cookie    string
		intercept bool
	}{
		{name: "Normal", path: "/guaclogin", method: "GET", cookie: knownSession, intercept: true},
		{name: "Index", path: "/", method: "GET", intercept: false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, host+tc.path, nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "session", Value: tc.cookie})
			}

			loginFunc := func(string, string) (string, error) {
				return "ok-token", nil
			}

			interceptor := guacamole.NewGuacTokenLoginEndpoint(us, ts, loginFunc)
			ok := interceptor.ValidRequest(req)
			if !ok {
				if tc.intercept {
					t.Fatalf("no interception, despite expected intercept")
				}

				return
			}

			emptyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				return
			})

			if !tc.intercept {
				t.Fatalf("intercepted despite not correct request")
			}

			w := httptest.NewRecorder()
			interceptor.Intercept(emptyHandler).ServeHTTP(w, req)

			resp := w.Result()
			var guactoken string
			for _, c := range resp.Cookies() {
				if c.Name == "GUAC_AUTH" {
					guactoken = c.Value
				}
			}

			if guactoken == "" {
				t.Fatalf("expected GUAC_AUTH cookie to be none empty")
			}
		})
	}
}
