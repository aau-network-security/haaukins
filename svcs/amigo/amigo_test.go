package amigo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aau-network-security/haaukins/store"
	pb "github.com/aau-network-security/haaukins/store/proto"
	mockserver "github.com/aau-network-security/haaukins/testing"
	"google.golang.org/grpc"
)

func TestVerifyFlag(t *testing.T) {
	// temporary events directory for NewStoreEvent
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	skey := "testing"
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

	ts, err := store.NewEventStore(store.EventConfig{
		Name:           "Test Event",
		Tag:            "test",
		Available:      1,
		Capacity:       2,
		Lab:            store.Lab{},
		StartedAt:      nil,
		FinishExpected: nil,
		FinishedAt:     nil,
		AllChallenges: map[string][]string{
			"test": []string{"test"},
		},
	}, tmp, client)

	var chal = store.ChildrenChalConfig{
		Tag:             "test",
		Name:            "Test Challenge",
		EnvVar:          "",
		StaticFlag:      "",
		Points:          10,
		TeamDescription: "",
		Category:        "",
	}

	addTeam := store.NewTeam("some@email.com", "somename", "password",
		"", "", "", time.Now().UTC(),
		map[string][]string{},
		map[string][]string{
			"test": []string{"test"},
		}, client)
	if err := ts.SaveTeam(addTeam); err != nil {
		t.Fatalf("expected no error when creating team")
	}
	flagValue := store.NewFlag().String()
	tag, _ := store.NewTag(string(chal.Tag))
	_, _ = addTeam.AddChallenge(store.Challenge{
		Tag:   tag,
		Name:  chal.Name,
		Value: flagValue,
	})

	team, err := ts.GetTeamByUsername("somename")
	if err != nil {
		t.Fatalf("unable to get the team by email: %v", err)
	}

	validToken, err := store.GetTokenForTeam([]byte(skey), team)
	if err != nil {
		t.Fatalf("unable to get token: %s", err)
	}
	validCookie := &http.Cookie{Name: "session", Value: validToken}

	tt := []struct {
		name   string
		input  string
		cookie *http.Cookie
		opts   []AmigoOpt
		err    string
	}{
		{
			name:   "too large",
			input:  `{"flag": "too-large"}`,
			cookie: validCookie,
			opts:   []AmigoOpt{WithMaxReadBytes(0)},
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
			input:  fmt.Sprintf(`{"flag": "%s", "tag": "%s"}`, flagValue, chal.Tag),
		},
		{
			name:   "already taken flag",
			cookie: validCookie,
			input:  fmt.Sprintf(`{"flag": "%s", "tag": "%s"}`, flagValue, chal.Tag),
			err:    fmt.Sprintf("Flag for challenge [ %s ] is already completed!", chal.Tag),
		},
	}

	type reply struct {
		Err    string `json:"error,omitempty"`
		Status string `json:"status"`
	}

	var challenges []store.ChildrenChalConfig
	challenges = append(challenges, chal)

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			am := NewAmigo(ts, challenges, "", nil, tc.opts...)
			srv := httptest.NewServer(am.Handler(Hooks{}, http.NewServeMux()))

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

func Test_checkTeamName(t *testing.T) {
	tt := []struct {
		input string
		err   error
	}{
		{
			input: `CoolTeamName`,
			err:   nil,
		},
		{
			input: `ThisIsAWayTooLongName`,
			err:   ErrTeamNameToLarge,
		},
		{
			input: `{":%s`,
			err:   ErrTeamNameCharacters,
		},
		{
			input: ``,
			err:   ErrTeamNameEmpty,
		},
	}
	for _, tc := range tt {
		got := checkTeamName(tc.input)
		if tc.err != got {
			t.Fatalf("Got %v, and wanted %v", got, tc.err)
		}
	}
}
