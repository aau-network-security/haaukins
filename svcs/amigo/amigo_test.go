package amigo_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aau-network-security/haaukins/svcs/amigo"
)

func TestVerifyFlag(t *testing.T) {
	tt := []struct {
		name  string
		input string
		opts  []amigo.AmigoOpt
		err   string
	}{
		{
			name:  "too large",
			input: `{"flag": "too-large"}`,
			opts:  []amigo.AmigoOpt{amigo.WithMaxReadBytes(0)},
			err:   "request body is too large",
		},
	}

	type reply struct {
		Err    string `json:"error"`
		Status string `json:"status"`
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			am := amigo.NewAmigo(tc.opts...)
			srv := httptest.NewServer(am.Handler())

			req, err := http.NewRequest("POST", srv.URL+"/flags/verify", bytes.NewBuffer([]byte(tc.input)))
			if err != nil {
				t.Errorf("could not create request: %v", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("could not perform request: %v", err)
			}
			defer resp.Body.Close()

			var r reply
			if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
				t.Errorf("unable to read json respolse: %v", err)
			}

			if tc.err != "" {
				if r.Err != tc.err {
					t.Errorf("unexpected error (%s), expected: %s", r.Err, tc.err)
				}
			}
		})
	}
}
