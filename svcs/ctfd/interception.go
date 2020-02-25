// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package ctfd

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/aau-network-security/haaukins/store"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

var (
	chalPathRegex       = regexp.MustCompile(`/chal/([0-9]+)`)
	DuplicateConsentErr = errors.New("Cannot have more than one consent checkbox")
	NoConsentErr        = errors.New("No consent given")

	selectorTmpl, _ = template.New("Selector").Parse(`
<label for="{{.Tag}}">{{.Label}}</label>
<select name="{{.Tag}}" class="form-control" required>
<option></option>{{range .Options}}
<option>{{.}}</option>{{end}}
</select>`)

	checkboxTmpl, _ = template.New("checkbox").Parse(`
<input class="form-check-input" type="checkbox" name="{{.Tag}}-checkbox" value="ok" checked>
<label class="form-check-label" for="{{.Tag}}-checkbox">
  {{.Text}}
</label>
`)
)

type Checkbox struct {
	Tag              string
	Text             string
	indicatesConcent bool
}

func NewCheckbox(tag string, text string, consent bool) *Checkbox {
	return &Checkbox{
		Tag:              tag,
		Text:             text,
		indicatesConcent: consent,
	}
}

func (c *Checkbox) Html() template.HTML {
	var out bytes.Buffer
	checkboxTmpl.Execute(&out, c)
	return template.HTML(out.String())
}

func (c *Checkbox) ReadMetadata(r *http.Request, team *store.Team) error {
	formName := fmt.Sprintf("%s-checkbox", c.Tag)
	v := r.FormValue(formName)
	if c.indicatesConcent && v == "" {
		return NoConsentErr
	}

	team.AddMetadata(c.Tag, v)

	delete(r.Form, formName)
	return nil
}

type Selector struct {
	Label   string
	Tag     string
	Options []string
	lookup  map[string]struct{}
}

func (s *Selector) Html() template.HTML {
	var out bytes.Buffer
	selectorTmpl.Execute(&out, s)
	return template.HTML(out.String())
}

func (s *Selector) ReadMetadata(r *http.Request, team *store.Team) error {
	v := r.FormValue(s.Tag)
	if v == "" {
		return fmt.Errorf("field \"%s\" cannot be empty", s.Label)
	}

	if _, ok := s.lookup[v]; !ok {
		return fmt.Errorf("invalid value for field \"%s\"", s.Label)
	}

	delete(r.Form, s.Tag)

	team.AddMetadata(s.Tag, v)

	return nil
}

func NewSelector(label string, tag string, options []string) *Selector {
	lookup := make(map[string]struct{})
	for _, opt := range options {
		lookup[opt] = struct{}{}
	}

	return &Selector{
		Label:   label,
		Tag:     tag,
		Options: options,
		lookup:  lookup,
	}
}

type Input interface {
	Html() template.HTML
	ReadMetadata(r *http.Request, team *store.Team) error
}

type InputRow struct {
	Class  string
	Inputs []Input
}

type ExtraFields struct {
	html           string
	inputs         []Input
	concentChecker *Checkbox
}

func NewExtraFields(rows []InputRow) (*ExtraFields, error) {
	var concentChecker *Checkbox

	var inputs []Input
	for _, row := range rows {
		for _, input := range row.Inputs {
			checkbox, ok := input.(*Checkbox)
			if ok && checkbox.indicatesConcent {
				if concentChecker != nil {
					return nil, DuplicateConsentErr
				}
				concentChecker = checkbox
				continue
			}
			inputs = append(inputs, input)
		}
	}

	var htmlRows []struct {
		Class  string
		Inputs []interface{}
	}
	for _, row := range rows {
		colsize := 12 / len(row.Inputs)

		var cols []interface{}
		for _, col := range row.Inputs {
			cols = append(cols, struct {
				Width int
				Html  template.HTML
			}{colsize, col.Html()})
		}

		htmlRow := struct {
			Class  string
			Inputs []interface{}
		}{
			Class:  row.Class,
			Inputs: cols,
		}
		htmlRows = append(htmlRows, htmlRow)
	}

	var htmlRaw bytes.Buffer
	if err := extraFieldsTmpl.Execute(&htmlRaw, htmlRows); err != nil {
		return nil, err
	}

	return &ExtraFields{
		concentChecker: concentChecker,
		inputs:         inputs,
		html:           htmlRaw.String(),
	}, nil
}

var (
	extraFieldsTmpl, _ = template.New("extra-fields").Parse(`
{{range .}}
<div class="{{.Class}} row">
	{{range .Inputs}}
	<div class="col-md-{{.Width}}">
		{{.Html}}
	</div>
	{{end}}
</div>
{{end}}`)
)

func (ef *ExtraFields) Html() string {
	return ef.html
}

func (ef *ExtraFields) ReadMetadata(r *http.Request, team *store.Team) []error {
	if ef.concentChecker != nil {
		if err := ef.concentChecker.ReadMetadata(r, team); err != nil {
			return nil
		}
	}

	var errs []error
	for _, input := range ef.inputs {
		if err := input.ReadMetadata(r, team); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

type signupInterception struct {
	extraFields *ExtraFields
}

func NewSignupInterception(ef *ExtraFields) *signupInterception {
	return &signupInterception{
		extraFields: ef,
	}

}

func (si *signupInterception) ValidRequest(r *http.Request) bool {
	if r.URL.Path == "/register" && r.Method == http.MethodGet {
		return true
	}

	return false
}

func (si *signupInterception) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recordAndServe(next, r, w, WithExtraFields(si.extraFields))
	})
}

type RegisterInterceptOpts func(*registerInterception)

func WithRegisterHooks(hooks ...func(*store.Team) error) RegisterInterceptOpts {
	return func(ri *registerInterception) {
		ri.preHooks = append(ri.preHooks, hooks...)
	}
}

func WithExtraRegisterFields(ef *ExtraFields) RegisterInterceptOpts {
	return func(ri *registerInterception) {
		ri.extraFields = ef
	}
}

func NewRegisterInterception(ts store.TeamStore, opts ...RegisterInterceptOpts) *registerInterception {
	ri := &registerInterception{
		teamStore: ts,
	}

	for _, opt := range opts {
		opt(ri)
	}

	return ri
}

type registerInterception struct {
	preHooks    []func(*store.Team) error
	teamStore   store.TeamStore
	extraFields *ExtraFields
}

func (*registerInterception) ValidRequest(r *http.Request) bool {
	if r.URL.Path == "/register" && r.Method == http.MethodPost {
		return true
	}

	return false
}

func (ri *registerInterception) Intercept(next http.Handler) http.Handler {
	teamFromRequest := func(r *http.Request) store.Team {
		name := r.FormValue("name")
		email := r.FormValue("email")
		pass := r.FormValue("password")
		return store.NewTeam(email, name, pass)
	}

	updateRequest := func(r *http.Request, t *store.Team) error {
		var err error
		for _, h := range ri.preHooks {
			if herr := h(t); herr != nil {
				err = herr
				break
			}
		}

		r.Form.Set("password", t.HashedPassword)

		// update body and content-length
		formdata := r.Form.Encode()
		r.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(formdata)))
		r.ContentLength = int64(len(formdata))

		return err
	}

	store := func(resp *http.Response, t store.Team) {
		var session string
		for _, c := range resp.Cookies() {
			if c.Name == "session" {
				session = c.Value
				break
			}
		}

		if session != "" {
			if err := ri.teamStore.CreateTeam(t); err != nil {
				log.Warn().
					Err(err).
					Str("email", t.Email).
					Str("name", t.Name).
					Msg("Unable to store new team")
				return
			}

			if err := ri.teamStore.CreateTokenForTeam(session, t); err != nil {
				log.Warn().
					Err(err).
					Str("email", t.Email).
					Str("name", t.Name).
					Msg("Unable to store session token for team")
				return
			}
		}
	}

	if ri.extraFields != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t := teamFromRequest(r)
			errs := ri.extraFields.ReadMetadata(r, &t)

			var mods []RespModifier
			if errs != nil {
				r.Form.Set("name", "")
				r.Form.Set("email", "")
				t.HashedPassword = ""

				mods = append(mods, WithRemoveErrors())
				mods = append(mods, WithAppendErrors(errs))
				mods = append(mods, WithExtraFields(ri.extraFields))
			}

			if err := updateRequest(r, &t); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("An error has occured, account could not be created\n\n"))
				w.Write([]byte(err.Error()))
				return

				//mods = append(mods, WithAppendErrors([]error{err}))
			}

			resp, _ := recordAndServe(next, r, w, mods...)
			store(resp, t)
		})
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := teamFromRequest(r)

		if err := updateRequest(r, &t); err != nil {
			resp, _ := recordAndServe(next, r, w, WithAppendErrors([]error{err}))
			store(resp, t)
			return
		}

		resp, _ := recordAndServe(next, r, w)
		store(resp, t)
	})
}

type challengeResp struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type checkFlagInterception struct {
	teamStore store.TeamStore
	flagPool  *FlagPool
}

func NewCheckFlagInterceptor(ts store.TeamStore, fp *FlagPool) *checkFlagInterception {
	return &checkFlagInterception{
		teamStore: ts,
		flagPool:  fp,
	}
}

func (*checkFlagInterception) ValidRequest(r *http.Request) bool {
	if r.Method == http.MethodPost && chalPathRegex.MatchString(r.URL.Path) {
		return true
	}

	return false
}

func (cfi *checkFlagInterception) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, err := cfi.getTeamFromSession(r)
		if err != nil {
			log.Warn().
				Err(err).
				Msg("Unable to get team based on session")
			return
		}

		matches := chalPathRegex.FindStringSubmatch("/" + r.URL.Path)
		chalNumStr := matches[1]
		cid, _ := strconv.Atoi(chalNumStr)

		originalFlag := r.FormValue("key")

		originalFlag = strings.TrimSpace(originalFlag)

		translatedFlag := cfi.flagPool.TranslateFlagForTeam(t, cid, originalFlag)

		r.Form.Set("key", translatedFlag)

		// update body and content-length
		formdata := r.Form.Encode()
		r.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(formdata)))
		r.ContentLength = int64(len(formdata))

		resp, body := recordAndServe(next, r, w)
		defer resp.Body.Close()

		var chal challengeResp
		if err := json.Unmarshal(body, &chal); err != nil {
			log.Warn().
				Err(err).
				Msg("Unable to read response from flag intercept")
			return
		}

		if strings.ToLower(chal.Message) == "correct" {
			tag, err := cfi.flagPool.GetTagByIdentifier(cid)
			if err != nil {
				log.Warn().
					Err(err).
					Int("challenge-id", cid).
					Msg("Unable to find challenge tag for identifier")
				return
			}

			err = t.SolveChallenge(tag, originalFlag)
			if err != nil {
				log.Warn().
					Err(err).
					Str("tag", string(tag)).
					Str("team-id", t.Id).
					Str("original", originalFlag).
					Str("translated", translatedFlag).
					Msg("Unable to solve challenge for team")
				return
			}

			err = cfi.teamStore.SaveTeam(t)
			if err != nil {
				log.Warn().
					Err(err).
					Str("tag", string(tag)).
					Str("team-id", t.Id).
					Msg("Unable to save team")
				return
			}

			log.Debug().
				Int("challenge-id", cid).
				Str("tag", string(tag)).
				Str("team-id", t.Id).
				Str("original", originalFlag).
				Str("translated", translatedFlag).
				Msg("Successfully solved challenge")
		}
	})
}

func (cfi *checkFlagInterception) getTeamFromSession(r *http.Request) (store.Team, error) {
	c, err := r.Cookie("session")
	if err != nil {
		return store.Team{}, fmt.Errorf("Unable to find session cookie")
	}

	session := c.Value
	t, err := cfi.teamStore.GetTeamByToken(session)
	log.Info().Str("team-id", t.Id).Msg("GetTeamByToken (Session cookie counted as token)")
	if err != nil {
		return store.Team{}, err
	}
	return t, nil
}

type loginInterception struct {
	teamStore store.TeamStore
}

func NewLoginInterceptor(ts store.TeamStore) *loginInterception {
	return &loginInterception{
		teamStore: ts,
	}
}

func (*loginInterception) ValidRequest(r *http.Request) bool {
	if r.URL.Path == "/login" && r.Method == http.MethodPost {
		return true
	}

	return false
}

func (li *loginInterception) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.FormValue("name")
		pass := r.FormValue("password")

		// skip admin user
		if name != "admin" {
			hashedPass := fmt.Sprintf("%x", sha256.Sum256([]byte(pass)))
			r.Form.Set("password", hashedPass)
		}

		// update body and content-length
		formdata := r.Form.Encode()
		r.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(formdata)))
		r.ContentLength = int64(len(formdata))

		resp, _ := recordAndServe(next, r, w)

		var session string
		for _, c := range resp.Cookies() {
			if c.Name == "session" {
				session = c.Value
				break
			}
		}

		var t store.Team
		var err error
		t, err = li.teamStore.GetTeamByEmail(name)
		if err != nil {
			t, err = li.teamStore.GetTeamByName(name)
		}

		if err != nil {
			log.Warn().
				Str("name", name).
				Msg("Unknown team with name/email")
			return
		}

		if session != "" {
			li.teamStore.CreateTokenForTeam(session, t)
		}
	})
}

var (
	errTmpl, _ = template.New("error").Parse(`
{{range .}}
<div class="alert alert-danger alert-dismissable" role="alert">
				  <span class="sr-only">Error:</span>
{{.Error}}
				  <button type="button" class="close" data-dismiss="alert" aria-label="Close"><span aria-hidden="true">×</span></button>
				</div>
{{end}}
`)
)

func WithRemoveErrors() RespModifier {
	return func(doc *goquery.Document) {
		doc.Find(".alert").Remove()
	}
}

func WithAppendErrors(errs []error) RespModifier {
	return func(doc *goquery.Document) {
		if errs == nil || len(errs) == 0 {
			return
		}

		var out bytes.Buffer
		errTmpl.Execute(&out, errs)
		errors := out.String()
		doc.Find("form.form-horizontal").BeforeHtml(errors)
	}
}

func WithExtraFields(ef *ExtraFields) RespModifier {
	html := ef.Html()

	return func(doc *goquery.Document) {
		doc.Find(".form-group").Last().AfterHtml(html)
	}
}

type RespModifier func(*goquery.Document)

func recordAndServe(next http.Handler, r *http.Request, w http.ResponseWriter, mods ...RespModifier) (*http.Response, []byte) {
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, r)
	for k, v := range rec.HeaderMap {
		if k == "Content-Length" {
			continue
		}
		w.Header()[k] = v
	}

	var rawBody bytes.Buffer
	writeDirectly := len(mods) == 0
	if writeDirectly {
		w.Header()["Content-Length"] = rec.HeaderMap["Content-Length"]
		w.WriteHeader(rec.Code)

		multi := io.MultiWriter(w, &rawBody)
		rec.Body.WriteTo(multi)

		return rec.Result(), rawBody.Bytes()
	}

	rec.Body.WriteTo(&rawBody)
	body := rawBody.Bytes()

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		log.Warn().
			Err(err).
			Msg("Unable to parse HTML as goquery.Document")
		return rec.Result(), body
	}

	for _, mod := range mods {
		mod(doc)
	}

	html, err := doc.Html()
	if err != nil {
		log.Warn().
			Err(err).
			Msg("Unable to get HTML of document")
	}

	body = []byte(html)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(rec.Code)

	_, err = w.Write(body)
	if err != nil {
		log.Warn().
			Err(err).
			Msg("Unable to write reponse")
	}

	return rec.Result(), body
}

func isTeamExists(name, email string, ri *registerInterception) bool {
	tByEmail, err := ri.teamStore.GetTeamByEmail(email)
	if err != nil {
		log.Error().Str("Error happened getting team in isTeamExists function", err.Error()).Msg("Error; ")
		return false
	}
	tByName, err := ri.teamStore.GetTeamByName(name)
	if err != nil {
		log.Error().Str("Error happened getting team in isTeamExists function", err.Error()).Msg("Error; ")
		return false
	}
	if (tByEmail.Name == name || tByEmail.Email == email) || (tByName.Name == name || tByName.Email == email) {
		return true
	}
	return false
}
