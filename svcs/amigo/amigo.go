package amigo

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"
	"unicode"

	logger "github.com/rs/zerolog/log"

	"github.com/aau-network-security/haaukins/store"
	"github.com/dgrijalva/jwt-go"
)

const (
	ID_KEY       = "I"
	TEAMNAME_KEY = "TN"
	signingKey   = "testing"
)

var (
	ErrReadBodyTooLarge     = errors.New("request body is too large")
	ErrUnauthorized         = errors.New("requires authentication")
	ErrInvalidTokenFormat   = errors.New("invalid token format")
	ErrInvalidFlag          = errors.New("invalid flag")
	ErrIncorrectCredentials = errors.New("Credentials does not match")
	wd                      = GetWd()
)

type siteInfo struct {
	EventName string
	Team      *team
	Content   interface{}
}

type team struct {
	Id   string
	Name string
}

type Amigo struct {
	maxReadBytes int64
	signingKey   []byte
	cookieTTL    int
	globalInfo   siteInfo
	challenges   []store.FlagConfig
	TeamStore    store.Event
	recaptcha    Recaptcha
}

type AmigoOpt func(*Amigo)

func WithMaxReadBytes(b int64) AmigoOpt {
	return func(am *Amigo) {
		am.maxReadBytes = b
	}
}

func WithEventName(eventName string) AmigoOpt {
	return func(am *Amigo) {
		am.globalInfo.EventName = eventName
	}
}

func NewAmigo(ts store.Event, chals []store.FlagConfig, reCaptchaKey string, opts ...AmigoOpt) *Amigo {
	am := &Amigo{
		maxReadBytes: 1024 * 1024,
		signingKey:   []byte(signingKey),
		challenges:   chals,
		cookieTTL:    int((7 * 24 * time.Hour).Seconds()), // A week
		TeamStore:    ts,
		globalInfo: siteInfo{
			EventName: "Test Event",
		},
		recaptcha: NewRecaptcha(reCaptchaKey),
	}

	for _, opt := range opts {
		opt(am)
	}

	return am
}

func (am *Amigo) getSiteInfo(w http.ResponseWriter, r *http.Request) siteInfo {
	info := am.globalInfo

	c, err := r.Cookie("session")
	if err != nil {
		return info
	}

	team, err := am.getTeamInfoFromToken(c.Value)
	if err != nil {
		http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: -1})
		return info
	}

	info.Team = team
	return info
}

type Hooks struct {
	AssignLab     func(t *store.Team) error
	ResetExercise func(t *store.Team, challengeTag string) error
	ResetFrontend func(t *store.Team) error
}

func (am *Amigo) Handler(hooks Hooks, guacHandler http.Handler) http.Handler {
	fd := newFrontendData(am.TeamStore, am.challenges...)
	go fd.runFrontendData()

	m := http.NewServeMux()

	m.HandleFunc("/", am.handleIndex())
	m.HandleFunc("/challenges", am.handleChallenges())
	m.HandleFunc("/teams", am.handleTeams())
	m.HandleFunc("/scoreboard", am.handleScoreBoard())
	m.HandleFunc("/signup", am.handleSignup(hooks.AssignLab))
	m.HandleFunc("/login", am.handleLogin())
	m.HandleFunc("/logout", am.handleLogout())
	m.HandleFunc("/scores", fd.handleConns())
	m.HandleFunc("/challengesFrontend", fd.handleConns())
	m.HandleFunc("/flags/verify", am.handleFlagVerify())
	m.HandleFunc("/reset/challenge", am.handleResetChallenge(hooks.ResetExercise))
	m.HandleFunc("/reset/frontend", am.handleResetFrontend(hooks.ResetFrontend))
	m.Handle("/guaclogin", guacHandler)
	m.Handle("/guacamole", guacHandler)
	m.Handle("/guacamole/", guacHandler)

	m.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir(wd+"/svcs/amigo/resources/public"))))

	return m
}

func (am *Amigo) handleIndex() http.HandlerFunc {
	tmpl, err := template.ParseFiles(
		wd+"/svcs/amigo/resources/private/base.tmpl.html",
		wd+"/svcs/amigo/resources/private/navbar.tmpl.html",
		wd+"/svcs/amigo/resources/private/index.tmpl.html",
	)
	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		data := am.getSiteInfo(w, r)
		if err := tmpl.Execute(w, data); err != nil {
			log.Println("template err index: ", err)
		}
	}
}

func (am *Amigo) handleChallenges() http.HandlerFunc {
	tmpl, err := template.ParseFiles(
		wd+"/svcs/amigo/resources/private/base.tmpl.html",
		wd+"/svcs/amigo/resources/private/navbar.tmpl.html",
		wd+"/svcs/amigo/resources/private/challenges.tmpl.html",
	)
	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/challenges" {
			http.NotFound(w, r)
			return
		}

		_, err := am.getTeamFromRequest(w, r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}

		data := am.getSiteInfo(w, r)
		if err := tmpl.Execute(w, data); err != nil {
			log.Println("template err index: ", err)
		}
	}
}

func (am *Amigo) handleTeams() http.HandlerFunc {
	tmpl, err := template.ParseFiles(
		wd+"/svcs/amigo/resources/private/base.tmpl.html",
		wd+"/svcs/amigo/resources/private/navbar.tmpl.html",
		wd+"/svcs/amigo/resources/private/teams.tmpl.html",
	)
	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/teams" {
			http.NotFound(w, r)
			return
		}

		data := am.getSiteInfo(w, r)
		if err := tmpl.Execute(w, data); err != nil {
			log.Println("template err index: ", err)
		}
	}
}

func (am *Amigo) handleScoreBoard() http.HandlerFunc {
	tmpl, err := template.ParseFiles(
		wd+"/svcs/amigo/resources/private/base.tmpl.html",
		wd+"/svcs/amigo/resources/private/navbar.tmpl.html",
		wd+"/svcs/amigo/resources/private/scoreboard.tmpl.html",
	)
	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/scoreboard" {
			http.NotFound(w, r)
			return
		}

		data := am.getSiteInfo(w, r)
		if err := tmpl.Execute(w, data); err != nil {
			log.Println("template err index: ", err)
		}
	}
}

func (am *Amigo) handleFlagVerify() http.HandlerFunc {
	type verifyFlagMsg struct {
		Tag  string `json:"tag"`
		Flag string `json:"flag"`
	}

	type replyMsg struct {
		Status string `json:"status"`
	}

	endpoint := func(w http.ResponseWriter, r *http.Request) {
		team, err := am.getTeamFromRequest(w, r)
		if err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		var msg verifyFlagMsg
		if err := safeReadJson(w, r, &msg, am.maxReadBytes); err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		flag, err := store.NewFlagFromString(msg.Flag)
		if err != nil {
			replyJson(http.StatusOK, w, errReply{ErrInvalidFlag.Error()})
			return
		}

		tag := store.Tag(msg.Tag)
		if err := team.VerifyFlag(store.Challenge{Tag: tag}, flag); err != nil {
			replyJson(http.StatusOK, w, errReply{err.Error()})
			return
		}

		replyJson(http.StatusOK, w, replyMsg{"ok"})
	}

	for _, mw := range []Middleware{JSONEndpoint, POSTEndpoint} {
		endpoint = mw(endpoint)
	}

	return endpoint
}

func (am *Amigo) handleSignup(hook func(t *store.Team) error) http.HandlerFunc {
	get := am.handleSignupGET()
	post := am.handleSignupPOST(hook)

	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			get(w, r)
			return

		case http.MethodPost:
			post(w, r)
			return
		}

		http.NotFound(w, r)
	}
}

func (am *Amigo) handleSignupGET() http.HandlerFunc {
	tmpl, err := template.ParseFiles(
		wd+"/svcs/amigo/resources/private/base.tmpl.html",
		wd+"/svcs/amigo/resources/private/navbar.tmpl.html",
		wd+"/svcs/amigo/resources/private/signup.tmpl.html",
	)
	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if err := tmpl.Execute(w, am.globalInfo); err != nil {
			log.Println("template err signup: ", err)
		}
	}
}

func (am *Amigo) handleSignupPOST(hook func(t *store.Team) error) http.HandlerFunc {
	tmpl, err := template.ParseFiles(
		wd+"/svcs/amigo/resources/private/base.tmpl.html",
		wd+"/svcs/amigo/resources/private/navbar.tmpl.html",
		wd+"/svcs/amigo/resources/private/signup.tmpl.html",
	)
	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	type signupData struct {
		Email       string
		TeamName    string
		Password    string
		SignupError string
	}

	readParams := func(r *http.Request) (signupData, error) {
		data := signupData{
			Email:    strings.TrimSpace(r.PostFormValue("email")),
			TeamName: strings.TrimSpace(r.PostFormValue("team-name")),
			Password: r.PostFormValue("password"),
		}

		if data.Email == "" {
			return data, fmt.Errorf("Email cannot be empty")
		}

		if data.TeamName == "" {
			return data, fmt.Errorf("Team Name cannot be empty")
		}

		if len(data.Password) <= 5 {
			return data, fmt.Errorf("Password needs to be at least six characters")
		}

		if data.Password != r.PostFormValue("password-repeat") {
			return data, fmt.Errorf("Password needs to match")
		}

		return data, nil
	}

	displayErr := func(w http.ResponseWriter, params signupData, err error) {
		tmplData := am.globalInfo
		params.SignupError = err.Error()
		tmplData.Content = params
		if err := tmpl.Execute(w, tmplData); err != nil {
			log.Println("template err signup: ", err)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, am.maxReadBytes)
		params, err := readParams(r)
		if err != nil {
			displayErr(w, params, err)
			return
		}

		if len(am.TeamStore.GetTeams()) == am.TeamStore.Capacity {
			displayErr(w, params, errors.New("capacity reached for this event"))
			return
		}
		// make the key empty for running haaukins on dev/local
		// making recaptcha place empty on config will disable verify
		if am.recaptcha.secret != "" {
			logger.Info().Msgf("Recaptcha is enabled on sign up page ")
			isValid := am.recaptcha.Verify(r.FormValue("g-recaptcha-response"))
			if !isValid {
				displayErr(w, params, errors.New("seems you are a robot"))
				return
			}
		}

		t := store.NewTeam(strings.TrimSpace(params.Email), strings.TrimSpace(params.TeamName), params.Password, "", "", "", nil)

		if err := am.TeamStore.SaveTeam(t); err != nil {
			displayErr(w, params, err)
			return
		}

		if err := am.loginTeam(w, r, t); err != nil {
			displayErr(w, params, err)
			return
		}
		token, err := store.GetTokenForTeam(am.signingKey, t)
		if err != nil {
			logger.Debug().Msgf("Error on getting token from amigo %s", token)
			return
		}

		if err := am.TeamStore.SaveTokenForTeam(token, t); err != nil {
			logger.Debug().Msgf("Create token for team error %s", err)
			return
		}

		if err := hook(t); err != nil { // assigning lab
			logger.Debug().Msgf("Problem in assing lab !! %s ", err)
		}
	}
}

func (am *Amigo) handleResetChallenge(resetHook func(t *store.Team, challengeTag string) error) http.HandlerFunc {

	type resetChallenge struct {
		Tag string `json:"tag"`
	}

	type replyMsg struct {
		Status string `json:"status"`
	}

	endpoint := func(w http.ResponseWriter, r *http.Request) {
		team, err := am.getTeamFromRequest(w, r)
		if err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		var msg resetChallenge
		if err := safeReadJson(w, r, &msg, am.maxReadBytes); err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		chalTag := getParentChallengeTag(msg.Tag)
		err = resetHook(team, chalTag)
		if err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		replyJson(http.StatusOK, w, replyMsg{"ok"})
	}

	for _, mw := range []Middleware{JSONEndpoint, POSTEndpoint} {
		endpoint = mw(endpoint)
	}

	return endpoint
}

func (am *Amigo) handleResetFrontend(resetFrontend func(t *store.Team) error) http.HandlerFunc {

	type replyMsg struct {
		Status string `json:"status"`
	}

	endpoint := func(w http.ResponseWriter, r *http.Request) {
		team, err := am.getTeamFromRequest(w, r)
		if err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		err = resetFrontend(team)
		if err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		replyJson(http.StatusOK, w, replyMsg{"ok"})
	}

	for _, mw := range []Middleware{JSONEndpoint, POSTEndpoint} {
		endpoint = mw(endpoint)
	}

	return endpoint
}

func (am *Amigo) handleLogin() http.HandlerFunc {
	get := am.handleLoginGET()
	post := am.handleLoginPOST()

	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			get(w, r)
			return

		case http.MethodPost:
			post(w, r)
			return
		}

		http.NotFound(w, r)
	}
}

func (am *Amigo) handleLoginGET() http.HandlerFunc {
	tmpl, err := template.ParseFiles(
		wd+"/svcs/amigo/resources/private/base.tmpl.html",
		wd+"/svcs/amigo/resources/private/navbar.tmpl.html",
		wd+"/svcs/amigo/resources/private/login.tmpl.html",
	)
	if err != nil {
		log.Println("error login tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if err := tmpl.Execute(w, am.globalInfo); err != nil {
			log.Println("template err login: ", err)
		}
	}
}

func (am *Amigo) handleLoginPOST() http.HandlerFunc {
	tmpl, err := template.ParseFiles(
		wd+"/svcs/amigo/resources/private/base.tmpl.html",
		wd+"/svcs/amigo/resources/private/navbar.tmpl.html",
		wd+"/svcs/amigo/resources/private/login.tmpl.html",
	)
	if err != nil {
		log.Println("error login tmpl: ", err)
	}

	type loginData struct {
		Username   string
		Password   string
		LoginError string
	}

	readParams := func(r *http.Request) (loginData, error) {
		data := loginData{
			Username: strings.TrimSpace(r.PostFormValue("username")),
			Password: r.PostFormValue("password"),
		}

		if data.Username == "" {
			return data, fmt.Errorf("Username cannot be empty")
		}

		if data.Password == "" {
			return data, fmt.Errorf("Password cannot be empty")
		}

		return data, nil
	}

	displayErr := func(w http.ResponseWriter, params loginData, err error) {
		tmplData := am.globalInfo
		params.LoginError = err.Error()
		tmplData.Content = params
		if err := tmpl.Execute(w, tmplData); err != nil {
			log.Println("template err login: ", err)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, am.maxReadBytes)
		params, err := readParams(r)
		if err != nil {
			displayErr(w, params, err)
			return
		}

		t, err := am.TeamStore.GetTeamByUsername(params.Username)
		if err != nil {
			displayErr(w, params, ErrIncorrectCredentials)
			return
		}

		if t.IsPasswordEqual(params.Password) == false {
			displayErr(w, params, ErrIncorrectCredentials)
			return
		}

		if err := am.loginTeam(w, r, t); err != nil {
			displayErr(w, params, err)
			return
		}
	}
}

func (am *Amigo) handleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: -1})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (am *Amigo) loginTeam(w http.ResponseWriter, r *http.Request, t *store.Team) error {
	token, err := store.GetTokenForTeam(am.signingKey, t)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{Name: "session", Value: token, MaxAge: am.cookieTTL})
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func (am *Amigo) getTeamFromRequest(w http.ResponseWriter, r *http.Request) (*store.Team, error) {
	c, err := r.Cookie("session")
	if err != nil {
		return nil, ErrUnauthorized
	}

	team, err := am.getTeamInfoFromToken(c.Value)
	if err != nil {
		http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: -1})
		return nil, err
	}

	t, err := am.TeamStore.GetTeamByID(team.Id)
	if err != nil {
		http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: -1})
		return nil, err
	}

	return t, nil
}

func safeReadJson(w http.ResponseWriter, r *http.Request, i interface{}, bytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, bytes)
	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(i); err != nil {
		switch err.Error() {
		case "http: request body too large":
			return ErrReadBodyTooLarge
		default:
			return err
		}
	}

	return nil
}

func replyJson(sc int, w http.ResponseWriter, i interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(sc)

	return json.NewEncoder(w).Encode(i)
}

type errReply struct {
	Error string `json:"error"`
}

func replyJsonRequestErr(w http.ResponseWriter, err error) error {
	return replyJson(http.StatusBadRequest, w, errReply{err.Error()})
}

func (am *Amigo) getTeamInfoFromToken(token string) (*team, error) {
	jwtToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return am.signingKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok || !jwtToken.Valid {
		return nil, ErrInvalidTokenFormat
	}

	id, ok := claims[ID_KEY].(string)
	if !ok {
		return nil, ErrInvalidTokenFormat
	}

	name, ok := claims[TEAMNAME_KEY].(string)
	if !ok {
		return nil, ErrInvalidTokenFormat
	}

	return &team{Id: id, Name: name}, nil
}

type Middleware func(http.HandlerFunc) http.HandlerFunc

func GETEndpoint(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

func POSTEndpoint(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

func JSONEndpoint(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

func getParentChallengeTag(child string) string {
	for _, c := range child {
		if unicode.IsDigit(c) {
			return child[:len(child)-2]
		}
	}
	return child
}

func GetWd() string {
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	return path
}
