package amigo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aau-network-security/haaukins"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/amigo"
	"github.com/google/uuid"
)

func TestVerifyFlag(t *testing.T) {
	skey := "someTestingKey"
	validFlag := haaukins.Flag(uuid.New())
	team := haaukins.NewTeam("test@aau.dk", "TesterTeam", "secretpass")
	team.AddChallenge(haaukins.Challenge{}, validFlag)

	validToken, err := amigo.GetTokenForTeam([]byte(skey), team)
	if err != nil {
		t.Fatalf("unable to get token: %s", err)
	}
	validCookie := &http.Cookie{Name: "session", Value: validToken}

	tt := []struct {
		name   string
		input  string
		cookie *http.Cookie
		opts   []amigo.AmigoOpt
		err    string
	}{
		{
			name:   "too large",
			input:  `{"flag": "too-large"}`,
			cookie: validCookie,
			opts:   []amigo.AmigoOpt{amigo.WithMaxReadBytes(0)},
			err:    "request body is too large",
		},
		{
			name:  "unauthorized",
			input: `{"flag": "some-flag"}`,
			err:   "requires authentication",
		},
		{
			name:   "valid flag",
			cookie: validCookie,
			input:  fmt.Sprintf(`{"flag": "%s"}`, uuid.UUID(validFlag).String()),
		},
		{
			name:   "unknown flag",
			cookie: validCookie,
			input:  `{"flag": "whatever-flag"}`,
			err:    "invalid flag",
		},
		{
			name:   "already taken flag",
			cookie: validCookie,
			input:  fmt.Sprintf(`{"flag": "%s"}`, uuid.UUID(validFlag).String()),
			err:    "Flag is already completed",
		},
	}

	type reply struct {
		Err    string `json:"error,omitempty"`
		Status string `json:"status"`
	}

	ts := store.NewTeamStore(team)
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			am := amigo.NewAmigo(ts, skey, tc.opts...)
			srv := httptest.NewServer(am.Handler())

			req, err := http.NewRequest("POST", srv.URL+"/flags/verify", bytes.NewBuffer([]byte(tc.input)))
			if err != nil {
				t.Fatalf("could not create request: %s", err)
			}
			req.Header.Add("Content-Type", "application/json")
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("could not perform request: %s", err)
			}
			defer resp.Body.Close()

			content, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("unable to read response body: %s", err)
			}

			var r reply
			if err := json.Unmarshal(content, &r); err != nil {
				t.Fatalf("unable to read json response (%s): %s", string(content), err)
			}

			if tc.err != "" {
				if r.Err != tc.err {
					t.Fatalf("unexpected error (%s), expected: %s", r.Err, tc.err)
				}
				return
			}

			if r.Err != "" {
				t.Fatalf("expected no errors to occur, but received: %s", r.Err)
			}

			if r.Status != "ok" {
				t.Fatalf("unexpected status: %s", r.Status)
			}
		})
	}
}
