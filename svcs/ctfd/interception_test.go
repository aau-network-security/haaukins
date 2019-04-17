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

	"github.com/PuerkitoBio/goquery"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/ctfd"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}

func TestRegisterInterception(t *testing.T) {
	endpoint := "http://sec02.lab.es.aau.dk/register"
	survey, _ := ctfd.NewExtraFields([]ctfd.InputRow{
		{
			Class: "form-group",
			Inputs: []ctfd.Input{
				ctfd.NewSelector("value1", "value1", []string{"1", "2", "3"}),
				ctfd.NewSelector("value2", "value2", []string{"a", "b"}),
			},
		},
		{
			Class: "form-check",
			Inputs: []ctfd.Input{
				ctfd.NewCheckbox("consent", "Can i has consent", true),
			},
		},
	})

	tt := []struct {
		name   string
		form   *url.Values
		opts   []ctfd.RegisterInterceptOpts
		params map[string]string
		err    string
	}{
		{
			name: "Normal",
			params: map[string]string{
				"email":    "some@email.dk",
				"name":     "username",
				"password": "some_password",
				"nonce":    "random_string",
			},
		},
		{
			name: "Normal with fields (default)",
			opts: []ctfd.RegisterInterceptOpts{ctfd.WithExtraRegisterFields(survey)},
			params: map[string]string{
				"email":            "some@email.dk",
				"name":             "username",
				"password":         "some_password",
				"nonce":            "random_string",
				"value1":           "2",
				"value2":           "b",
				"consent-checkbox": "ok",
			},
		},
		{
			name: "Normal with fields (disagree)",
			opts: []ctfd.RegisterInterceptOpts{ctfd.WithExtraRegisterFields(survey)},
			params: map[string]string{
				"email":    "some@email.dk",
				"name":     "username",
				"password": "some_password",
				"nonce":    "random_string",
			},
		},
		{
			name: "Missing survey (1)",
			opts: []ctfd.RegisterInterceptOpts{ctfd.WithExtraRegisterFields(survey)},
			params: map[string]string{
				"email":            "some@email.dk",
				"name":             "username",
				"password":         "some_password",
				"nonce":            "random_string",
				"value1":           "3",
				"consent-checkbox": "ok",
			},
			err: `field "value2" cannot be empty`,
		},
		{
			name: "Missing survey (2)",
			opts: []ctfd.RegisterInterceptOpts{ctfd.WithExtraRegisterFields(survey)},
			params: map[string]string{
				"email":            "some@email.dk",
				"name":             "username",
				"password":         "some_password",
				"nonce":            "random_string",
				"value2":           "b",
				"consent-checkbox": "ok",
			},
			err: `field "value1" cannot be empty`,
		},
		{
			name: "Incorrect value survey",
			opts: []ctfd.RegisterInterceptOpts{ctfd.WithExtraRegisterFields(survey)},
			params: map[string]string{
				"email":            "some@email.dk",
				"name":             "username",
				"password":         "some_password",
				"nonce":            "random_string",
				"value1":           "meow",
				"value2":           "b",
				"consent-checkbox": "ok",
			},
			err: `invalid value for field "value1"`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			f := url.Values{}
			for k, v := range tc.params {
				f.Add(k, v)
			}

			req := httptest.NewRequest("POST", endpoint, strings.NewReader(f.Encode()))
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			cl := req.ContentLength

			ts := store.NewTeamStore()
			var ranPreHook bool
			pre := func(*store.Team) error { ranPreHook = true; return nil }

			interceptor := ctfd.NewRegisterInterception(ts, append(tc.opts, ctfd.WithRegisterHooks(pre))...)
			ok := interceptor.ValidRequest(req)
			if !ok {
				t.Fatalf("no interception, despite expected intercept")
			}

			receivedValues := map[string]string{}
			var postCl int64
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.ParseForm()
				for k, v := range r.Form {
					receivedValues[k] = v[0]
				}

				postCl = req.ContentLength

				expiration := time.Now().Add(365 * 24 * time.Hour)
				cookie := http.Cookie{Name: "session", Value: "secret-cookie", Expires: expiration}
				http.SetCookie(w, &cookie)

				w.Write([]byte(`<form class="form-horizontal"></form>`))

				return
			})

			w := httptest.NewRecorder()
			interceptor.Intercept(testHandler).ServeHTTP(w, req)
			resp := w.Result()

			doc, err := goquery.NewDocumentFromReader(resp.Body)
			if err != nil {
				t.Fatalf("unable to read document from recorded response")
			}

			displayErrors := map[string]struct{}{}
			doc.Find(".alert").Each(func(i int, s *goquery.Selection) {
				s.Children().Remove()
				errMsg := strings.TrimSpace(s.Text())
				displayErrors[errMsg] = struct{}{}
			})

			if tc.err != "" {
				if _, ok := displayErrors[tc.err]; !ok {
					t.Fatalf("expected error (%s), but received none", tc.err)
				}

				for _, k := range []string{"name", "password", "email"} {
					v, ok := receivedValues[k]
					if !ok {
						t.Fatalf("expected to receive value %s on failure", k)
					}

					if v != "" {
						t.Fatalf("expected key (%s), to be empty, but received: %s", k, v)
					}
				}

				return
			}

			if len(displayErrors) > 0 {
				t.Fatalf("received display error(s), but expected none: %v", displayErrors)
			}

			for _, k := range []string{"name", "nonce", "email"} {
				v, ok := receivedValues[k]
				if !ok {
					t.Fatalf("expected to receive value %s", k)
				}

				if orgV := f.Get(k); v != orgV {
					t.Fatalf("expected %s to be \"%s\", but received: %s", k, orgV, v)
				}

				delete(receivedValues, k)
			}

			orgPassword := f.Get("password")
			if password := receivedValues["password"]; orgPassword == password {
				t.Fatalf("expected password to be changed (org: %s), received: %s", orgPassword, password)
			}
			delete(receivedValues, "password")

			if len(receivedValues) > 0 {
				var keys []string
				for k, _ := range receivedValues {
					keys = append(keys, k)
				}

				t.Fatalf("received unexpected keys: %v", keys)
			}

			if cl == postCl {
				t.Fatalf("expected content-length (pre: %d) to change after interception, received: %d", cl, postCl)
			}

			var session string
			for _, c := range resp.Cookies() {
				if c.Name == "session" {
					session = c.Value
				}
			}

			if session == "" {
				t.Fatalf("expected session to be none empty")
			}

			_, err = ts.GetTeamByToken(session)
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

func TestSelectorHtml(t *testing.T) {
	s := ctfd.NewSelector("Age", "age", []string{"0-14", "15-21", "22-30", "30-50", "51+"})
	htmlStr := s.Html()

	doc, err := goquery.NewDocumentFromReader(
		strings.NewReader(string(htmlStr)),
	)
	if err != nil {
		t.Fatalf("unable to read html: %s", err)
	}

	n := len(s.Options) + 1 // adding one for default element
	if count := doc.Find("option").Size(); count != n {
		t.Fatalf("expected %d option elements, but received: %d", n, count)
	}

}

func TestSelectorReadMetadata(t *testing.T) {
	s := ctfd.NewSelector("Age", "age", []string{"0-14", "15-21", "22-30", "30-50", "51+"})

	tt := []struct {
		name string
		form *url.Values
		err  string
	}{
		{name: "Normal", form: &url.Values{"age": {"0-14"}}},
		{name: "No values", err: `field "Age" cannot be empty`},
		{name: "Invalid value", err: `invalid value for field "Age"`, form: &url.Values{"age": {"abc"}}},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "http://test.com", nil)
			if tc.form != nil {
				values := *tc.form
				req = httptest.NewRequest("POST", "http://test.com", strings.NewReader(values.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			}

			var team store.Team
			err := s.ReadMetadata(req, &team)
			if err != nil {
				if tc.err != "" {
					if err.Error() != tc.err {

						t.Fatalf("expected error (%s), but received: %s", tc.err, err)
					}

					return
				}

				t.Fatalf("expected no error but received: %s", err)
				return
			}

			if tc.err != "" {
				t.Fatalf("expected error (%s), but received none", tc.err)
				return
			}

			if _, ok := team.Metadata["age"]; !ok {
				t.Fatalf("expected metadata to be read")
			}
		})
	}
}

func TestCheckboxHtml(t *testing.T) {
	cbox := ctfd.NewCheckbox("agree", "I agree", true)
	htmlStr := cbox.Html()

	doc, err := goquery.NewDocumentFromReader(
		strings.NewReader(string(htmlStr)),
	)
	if err != nil {
		t.Fatalf("unable to read html: %s", err)
	}

	if doc.Find("input[class=form-check-input]") == nil {
		t.Fatalf("expected input with class 'form-check-input' to exist, but it does not")
	}
}

func TestCheckboxReadMetadata(t *testing.T) {
	tt := []struct {
		name            string
		form            *url.Values
		expectedConsent bool
	}{
		{
			name:            "Normal",
			form:            &url.Values{"consent-checkbox": {"ok"}},
			expectedConsent: true,
		},
		{
			name:            "No consent given",
			form:            &url.Values{"consent-checkbox": {""}},
			expectedConsent: false,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cbox := ctfd.NewCheckbox("consent", "I agree", true)

			req := httptest.NewRequest("POST", "http://test.com", nil)
			if tc.form != nil {
				values := *tc.form
				req = httptest.NewRequest("POST", "http://test.com", strings.NewReader(values.Encode()))
				req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			}

			var team store.Team
			err := cbox.ReadMetadata(req, &team)
			if (err != nil && tc.expectedConsent) && (err == nil && !tc.expectedConsent) {
				t.Fatalf("unexpected error: %s", err)
			}

			if team.DataCollection() != tc.expectedConsent {
				t.Fatalf("expected consent to be '%t', but got '%t'", tc.expectedConsent, team.DataCollection())
			}
		})
	}
}
