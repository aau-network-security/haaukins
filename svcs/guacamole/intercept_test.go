// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package guacamole_test

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aau-network-security/haaukins/store"

	pb "github.com/aau-network-security/haaukins/store/proto"
	"github.com/aau-network-security/haaukins/svcs/amigo"
	"github.com/aau-network-security/haaukins/svcs/guacamole"
	mockserver "github.com/aau-network-security/haaukins/testing"
	"google.golang.org/grpc"
)

func TestGuacLoginTokenInterceptor(t *testing.T) {
	// temporary events directory for NewStoreEvent
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	dialer, close := mockserver.Create()
	defer close()

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithDialer(dialer),
		grpc.WithInsecure(),
	)
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewStoreClient(conn)

	host := "http://sec02.lab.es.aau.dk"
	knownSession := "some_known_session"

	ts, _ := store.NewEventStore(store.EventConfig{
		Name:           "Test Event",
		Tag:            "test",
		Available:      1,
		Capacity:       2,
		Lab:            store.Lab{},
		StartedAt:      nil,
		FinishExpected: nil,
		FinishedAt:     nil,
	}, tmp, client)

	team := store.NewTeam("some@email.com", "some name", "password",
		"", "", "", time.Now().UTC(), map[string][]string{}, map[string][]string{}, client)

	if err := ts.SaveTeam(team); err != nil {
		t.Fatalf("expected to be able to create team")
	}

	if err := ts.SaveTokenForTeam(knownSession, team); err != nil {
		t.Fatalf("expected to be able to create token for team")
	}

	us := guacamole.NewGuacUserStore()
	us.CreateUserForTeam(team.ID(), guacamole.GuacUser{Username: "some-user", Password: "some-pass"})

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

			interceptor := guacamole.NewGuacTokenLoginEndpoint(us, ts, amigo.NewAmigo(ts, nil, "", nil), loginFunc)
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
