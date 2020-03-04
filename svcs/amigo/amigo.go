package amigo

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"text/template"
	"time"

	"github.com/aau-network-security/haaukins"
	"github.com/aau-network-security/haaukins/store"
	"github.com/dgrijalva/jwt-go"
	logger "github.com/rs/zerolog/log"
)

const (
	ID_KEY       = "I"
	TEAMNAME_KEY = "TN"
)

var (
	ErrReadBodyTooLarge     = errors.New("request body is too large")
	ErrUnauthorized         = errors.New("requires authentication")
	ErrInvalidTokenFormat   = errors.New("invalid token format")
	ErrInvalidFlag          = errors.New("invalid flag")
	ErrIncorrectCredentials = errors.New("Credentials does not match")
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
	challenges   []haaukins.Challenge
	TeamStore    store.TeamStore
}

type AmigoOpt func(*Amigo)

func WithMaxReadBytes(b int64) AmigoOpt {
	return func(am *Amigo) {
		am.maxReadBytes = b
	}
}
func WithEventName(eventName string) AmigoOpt {
	return  func (am *Amigo){
		am.globalInfo.EventName = eventName
	}
}

func NewAmigo(ts store.TeamStore, chals []haaukins.Challenge, key string, opts ...AmigoOpt) *Amigo {
	am := &Amigo{
		maxReadBytes: 1024 * 1024,
		signingKey:   []byte(key),
		challenges:   chals,
		cookieTTL:    int((7 * 24 * time.Hour).Seconds()), // A week
		TeamStore:    ts,
		globalInfo: siteInfo{
			EventName: "Test Event",
		},
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

func (am *Amigo) Handler(hook func(t *haaukins.Team) error,guacHandler http.Handler ) http.Handler {
	sb := newScoreboard(am.TeamStore, am.challenges...)
	go sb.run()
	m := http.NewServeMux()

	m.HandleFunc("/", am.handleIndex())
	m.HandleFunc("/challenges", am.handleChallenges())
	m.HandleFunc("/signup", am.handleSignup(hook))
	m.HandleFunc("/login", am.handleLogin())
	m.HandleFunc("/logout", am.handleLogout())
	m.HandleFunc("/scores", sb.handleConns())
	m.HandleFunc("/flags/verify", am.handleFlagVerify())
	m.Handle("/guaclogin", guacHandler)
	m.Handle("/guacamole", guacHandler)
	m.Handle("/guacamole/", guacHandler)


	m.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir("/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/public"))))

	return m
}



func (am *Amigo) handleIndex() http.HandlerFunc {
	tmpl, err := template.ParseFiles(
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/base.tmpl.html",
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/navbar.tmpl.html",
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/index.tmpl.html",
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
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/base.tmpl.html",
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/navbar.tmpl.html",
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/index.tmpl.html",
	)
	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/challenges" {
			http.NotFound(w, r)
			return
		}

		data := am.getSiteInfo(w, r)
		if err := tmpl.Execute(w, data); err != nil {
			log.Println("template err index: ", err)
		}
	}
}

/*func getChallenges(t *store.ExerciseStore) TeamRow {
	chals := t.GetExercisesByTags()
	completions := make([]*time.Time, len(chals))
	for i, chal := range chals {
		completions[i] = chal.CompletedAt
	}

	return TeamRow{
		Id:          t.ID(),
		Name:        t.Name(),
		Completions: completions,
	}
}
*/
func (am *Amigo) handleFlagVerify() http.HandlerFunc {
	type verifyFlagMsg struct {
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

		flag, err := haaukins.NewFlagFromString(msg.Flag)
		if err != nil {
			replyJson(http.StatusOK, w, errReply{ErrInvalidFlag.Error()})
			return
		}

		if err := team.VerifyFlag(flag); err != nil {
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

func (am *Amigo) handleSignup(hook func(t *haaukins.Team) error) http.HandlerFunc {
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
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/base.tmpl.html",
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/navbar.tmpl.html",
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/signup.tmpl.html",
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

func (am *Amigo) handleSignupPOST(hook func(t *haaukins.Team) error) http.HandlerFunc {
	tmpl, err := template.ParseFiles(
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/base.tmpl.html",
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/navbar.tmpl.html",
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/signup.tmpl.html",
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
			Email:    r.PostFormValue("email"),
			TeamName: r.PostFormValue("team-name"),
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


		t := haaukins.NewTeam(params.Email, params.TeamName, params.Password)

		if err := am.TeamStore.SaveTeam(t); err != nil {
			displayErr(w, params, err)
			return
		}

		if err := am.loginTeam(w, r, t); err != nil {
			displayErr(w, params, err)
			return
		}
		token,err:= GetTokenForTeam(am.signingKey, t)
		if err !=nil {
			logger.Debug().Msgf("Error on getting token from amigo %s", token)
			return
		}
		logger.Debug().Msgf("TOKEN FROM GETTOKENFORTEAM FUNC : %s", token)
		//if err := am.TeamStore.CreateTokenForTeam(token,t); err !=nil {
		//	logger.Debug().Msgf("Create Token For TEAM ERROR ! %s", err)
		//}
		team,err := am.TeamStore.CreateTokenForTeam(token,t)
		if err != nil {
			logger.Debug().Msgf("Create token for team error %s",err)
			return
		}
		logger.Debug().Msgf("team toke from create token for team : %s", )

		//
		//if err:= am.TeamStore.CreateTokenForTeam(session.Value,t); err!=nil {
		//	logger.Debug().Msgf("Token could not created  for team %s error %s", t.Name(),token)
		//	return
		//}
		// assign lab !!!
		if err := hook(team); err != nil { // assigning lab
			logger.Debug().Msgf("Problem in assing lab !! %s ", err)
		}
	}
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
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/base.tmpl.html",
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/navbar.tmpl.html",
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/login.tmpl.html",
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
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/base.tmpl.html",
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/navbar.tmpl.html",
		"/home/ahmet/haaukins_main/haaukins/svcs/amigo/resources/private/login.tmpl.html",
	)
	if err != nil {
		log.Println("error login tmpl: ", err)
	}

	type loginData struct {
		Email      string
		Password   string
		LoginError string
	}

	readParams := func(r *http.Request) (loginData, error) {
		data := loginData{
			Email:    r.PostFormValue("email"),
			Password: r.PostFormValue("password"),
		}

		if data.Email == "" {
			return data, fmt.Errorf("Email cannot be empty")
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

		t, err := am.TeamStore.GetTeamByEmail(params.Email)
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

func (am *Amigo) loginTeam(w http.ResponseWriter, r *http.Request, t *haaukins.Team) error {
	token, err := GetTokenForTeam(am.signingKey, t)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{Name: "session", Value: token, MaxAge: am.cookieTTL})
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func (am *Amigo) getTeamFromRequest(w http.ResponseWriter, r *http.Request) (*haaukins.Team, error) {
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

func GetTokenForTeam(key []byte, t *haaukins.Team) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		ID_KEY:       t.ID(),
		TEAMNAME_KEY: t.Name(),
	})
	tokenStr, err := token.SignedString(key)
	if err != nil {
		return "", err
	}
	return tokenStr, nil
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
