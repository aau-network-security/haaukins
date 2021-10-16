package amigo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	wg "github.com/aau-network-security/haaukins/network/vpn"

	logger "github.com/rs/zerolog/log"

	"github.com/aau-network-security/haaukins/store"
	jwt "github.com/golang-jwt/jwt/v4"
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
	ErrTeamNameEmpty        = errors.New("Team name can NOT be empty")
	ErrTeamNameToLarge      = errors.New("Team name is too long")
	ErrTeamNameCharacters   = errors.New("Team name should NOT contain non-alphanumeric characters")
	ErrEmailEmpty           = errors.New("Email can NOT be empty")
	ErrEmailToLarge         = errors.New("Email is NOT within the defined character limit")
	ErrEmailCharacters      = errors.New("Non alphabetic characters are NOT allowed in email address such as - , { [ _   ")
	ErrProtectedEvent       = errors.New("UNABLE TO SIGNUP: WRONG SECRET KEY FOR PROTECTED EVENT ! \n ASK EVENT ADMINISTRATOR FOR SECRET KEY FOR THIS EVENT ! ")
	teamNameRegex           = "^[A-Za-z0-9]+$"
	emailRegex              = "^[a-zA-Z0-9.!#$%&'*+^\\{|}~]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$"
	wd                      = GetWd()
	passMaxLength           = 20
	passMinLength           = 6
	emailMaxLength          = 30
	teamNameMaxLength       = 20
)

type Hosts struct {
	Domain string
	IP     string
}

type siteInfo struct {
	EventName     string
	Team          *team
	EventSecret   string // this will be the secret value which will be used on signup !
	IsVPN         int32
	IsSecretEvent bool
	Content       interface{}
	Hosts         []Hosts
	Notification  Notification
	LabSubnet     string
	SignupPaused  bool // this is used when add challenge is executed in the platform
}

type team struct {
	Id   string
	Name string
}

type Notification struct {
	Message       string
	LoggedInUsers bool
}

type Amigo struct {
	maxReadBytes int64
	signingKey   []byte
	cookieTTL    int
	globalInfo   siteInfo
	challenges   []store.ChildrenChalConfig
	TeamStore    store.Event
	recaptcha    Recaptcha
	wgClient     wg.WireguardClient
	notification Notification
	FrontEndData *FrontendData
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

func NewAmigo(ts store.Event, chals []store.ChildrenChalConfig, reCaptchaKey string, wgClient wg.WireguardClient, opts ...AmigoOpt) *Amigo {

	am := &Amigo{
		maxReadBytes: 1024 * 1024,
		signingKey:   []byte(signingKey),
		challenges:   chals,
		cookieTTL:    int((7 * time.Hour).Seconds()), // Less than inactivity duration on daemon
		TeamStore:    ts,
		globalInfo: siteInfo{
			EventName:     "Test Event",
			EventSecret:   ts.SecretKey,
			IsSecretEvent: ts.SecretKey != "",
		},
		recaptcha: NewRecaptcha(reCaptchaKey),
		wgClient:  wgClient,
	}

	for _, opt := range opts {
		opt(am)
	}

	return am
}

func (am *Amigo) SetNotification(n Notification) {
	am.globalInfo.Notification = n
}

func (am *Amigo) PauseSignup(pause bool) {
	am.globalInfo.SignupPaused = pause
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
	info.IsVPN = am.TeamStore.OnlyVPN
	info.Team = team
	return info
}

type Hooks struct {
	AssignLab         func(t *store.Team) error
	ResetExercise     func(t *store.Team, challengeTag string) error
	StartStopExercise func(t *store.Team, challengeTag string, state bool) error
	ResetFrontend     func(t *store.Team) error
	ResumeTeamLab     func(t *store.Team) error
}

func (am *Amigo) Handler(hooks Hooks, guacHandler http.Handler) http.Handler {
	fd := NewFrontendData(am.TeamStore, am.challenges...)
	go fd.RunFrontendData()
	am.FrontEndData = fd
	m := http.NewServeMux()

	m.HandleFunc("/", am.handleIndex())
	m.HandleFunc("/info", am.handleInfo())
	m.HandleFunc("/challenges", am.handleChallenges())
	m.HandleFunc("/teams", am.handleTeams())
	m.HandleFunc("/hosts", am.handleHostsInformation())
	m.HandleFunc("/scoreboard", am.handleScoreBoard())
	m.HandleFunc("/signup", am.handleSignup(hooks.AssignLab))
	m.HandleFunc("/login", am.handleLogin(hooks.ResumeTeamLab))
	m.HandleFunc("/logout", am.handleLogout())
	m.HandleFunc("/scores", fd.handleConns())
	m.HandleFunc("/challengesFrontend", fd.handleConns())
	m.HandleFunc("/flags/verify", am.handleFlagVerify(hooks.StartStopExercise))
	m.HandleFunc("/reset/challenge", am.handleResetChallenge(hooks.ResetExercise))
	m.HandleFunc("/manage/challenge", am.handleStartStopChallenge(hooks.StartStopExercise))
	m.HandleFunc("/reset/frontend", am.handleResetFrontend(hooks.ResetFrontend))
	m.HandleFunc("/vpn/status", am.handleVPNStatus(hooks.AssignLab))
	m.HandleFunc("/vpn/download", am.handleVPNFiles())
	m.HandleFunc("/get/labsubnet", am.handleLabInfo())
	if am.TeamStore.OnlyVPN == 0 || am.TeamStore.OnlyVPN == 2 {
		m.Handle("/guaclogin", am.handleGuacConnection(hooks.AssignLab, guacHandler))
		m.Handle("/guacamole", guacHandler)
		m.Handle("/guacamole/", guacHandler)
	}

	m.Handle("/assets/", http.StripPrefix("/assets", http.FileServer(http.Dir(wd+"/svcs/amigo/resources/public"))))
	return m
}

func (am *Amigo) GetFrontendData() *FrontendData {
	return am.FrontEndData
}

func (am *Amigo) handleIndex() http.HandlerFunc {
	indexTemplate := wd + "/svcs/amigo/resources/private/index.tmpl.html"
	tmpl, err := parseTemplates(indexTemplate)
	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
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

func (am *Amigo) handleHostsInformation() http.HandlerFunc {
	type data struct {
		hostinfo string
	}
	hostsTemplate := wd + "/svcs/amigo/resources/private/hosts.tmpl.html"
	tmpl, err := parseTemplates(hostsTemplate)
	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		if r.URL.Path != "/hosts" {
			http.NotFound(w, r)
			return
		}

		t, err := am.getTeamFromRequest(w, r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		data := am.getSiteInfo(w, r)
		hosts := t.GetHostsInfo()
		var hostInfo []Hosts
		if hosts != nil {
			for _, r := range hosts {
				record := strings.Split(r, "\t")
				hostInfo = append(hostInfo, Hosts{IP: record[0], Domain: record[1]})
			}
			data.Hosts = hostInfo
		}
		if err := tmpl.Execute(w, data); err != nil {
			log.Println("template err index: ", err)
		}
	}

}

func (am *Amigo) handleGuacConnection(hook func(t *store.Team) error, next http.Handler) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		team, err := am.getTeamFromRequest(w, r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !team.IsLabAssigned() {
			if err := hook(team); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(waitingHTMLTemplate))
				return
			}
		}

		next.ServeHTTP(w, r)
	}
}

func (am *Amigo) handleInfo() http.HandlerFunc {
	infoTemplate := wd + "/svcs/amigo/resources/private/info.tmpl.html"
	tmpl, err := parseTemplates(infoTemplate)
	if err != nil {
		log.Println("error info tmpl: ", err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		if r.URL.Path != "/info" {
			http.NotFound(w, r)
			return
		}
		t, err := am.getTeamFromRequest(w, r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		data := am.getSiteInfo(w, r)
		hosts := t.GetHostsInfo()
		var ip string
		data.LabSubnet = "LAB IS NOT ASSIGNED YET"
		if len(hosts) != 0 {
			for _, r := range hosts {
				record := strings.Split(r, "\t")
				ip = record[0]
				break
			}
			ipParts := strings.Split(ip, ".")
			data.LabSubnet = fmt.Sprintf("%s.%s.%s.%s", ipParts[0], ipParts[1], ipParts[2], "0/24")
		}

		if err := tmpl.Execute(w, data); err != nil {
			log.Println("info template err : ", err)
		}
	}
}

func (am *Amigo) handleChallenges() http.HandlerFunc {
	chalsTemplate := wd + "/svcs/amigo/resources/private/challenges.tmpl.html"
	tmpl, err := parseTemplates(chalsTemplate)
	if err != nil {
		log.Println("error challenges tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

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

func (am *Amigo) handleVPNStatus(hook func(t *store.Team) error) http.HandlerFunc {
	// data to be sent
	type vpnStatus struct {
		VPNConfID string `json:"vpnConnID"`
		Status    string `json:"status"` // this could be returned to stream
	}
	ctx := context.Background()

	endpoint := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		team, err := am.getTeamFromRequest(w, r)
		if err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		if !team.IsLabAssigned() {
			if err := hook(team); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				return
			}
		}

		vpnConfig := team.GetVPNConn()
		teamVPNKeys := team.GetVPNKeys()

		if len(vpnConfig) == 0 {
			replyJsonRequestErr(w, fmt.Errorf("Error, no vpn information found on on team err %v", err))
		}
		//eventTag := string(am.TeamStore.Tag)
		var listOfStatus []vpnStatus
		// status of vpn should be retrieved from wg client. for PoC it is ok to write ok.

		for i, _ := range vpnConfig {
			id := fmt.Sprintf("conn")
			resp, err := am.wgClient.GetPeerStatus(ctx, &wg.PeerStatusReq{
				NicName:   string(am.TeamStore.Tag),
				PublicKey: teamVPNKeys[i],
			})
			status := vpnStatus{VPNConfID: id + "_" + strconv.Itoa(i), Status: "N/U"}
			if err != nil {
				log.Printf("Error on retrieving back information from wg %s", err.Error())
				replyJsonRequestErr(w, err)
				return
			}
			if resp.Status {
				status.Status = "USED"
			}
			listOfStatus = append(listOfStatus, status)

		}

		replyJson(http.StatusOK, w, listOfStatus)
	}
	for _, mw := range []Middleware{JSONEndpoint, POSTEndpoint} {
		endpoint = mw(endpoint)
	}

	return endpoint
}

// handleVPNFiles will give chance to download their configuration files
func (am *Amigo) handleVPNFiles() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		type vpnStatus struct {
			VPNConfID string `json:"vpnConnID"`
			Status    string `json:"status"` // this could be returned to stream
		}
		team, err := am.getTeamFromRequest(w, r)
		if err != nil {
			replyJsonRequestErr(w, err)
			return
		}
		vpnConfig := team.GetVPNConn()
		if len(vpnConfig) == 0 {
			replyJsonRequestErr(w, fmt.Errorf("Error, no vpn information found on on team err %v", err))
		}
		var vpnConn vpnStatus
		if err := safeReadJson(w, r, &vpnConn, am.maxReadBytes); err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		confID, err := strconv.Atoi(strings.Split(vpnConn.VPNConfID, "_")[1])
		if err != nil {
			replyJsonRequestErr(w, err)
		}

		//log.Printf("Trunced conf id %d", confID)
		writeConfig := func(id int) {
			w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fmt.Sprintf("conn_%d.conf", id)))
			b := strings.NewReader(vpnConfig[id])
			//stream the body to the client without fully loading it into memory
			io.Copy(w, b)
		}

		switch confID {
		case 0:
			writeConfig(0)
		case 1:
			writeConfig(1)
		case 2:
			writeConfig(2)
		case 3:
			writeConfig(3)
		}

	}
}

func (am *Amigo) handleTeams() http.HandlerFunc {
	teamsTemplate := wd + "/svcs/amigo/resources/private/teams.tmpl.html"
	tmpl, err := parseTemplates(teamsTemplate)

	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
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
	scoreBoardTemplate := wd + "/svcs/amigo/resources/private/scoreboard.tmpl.html"
	tmpl, err := parseTemplates(scoreBoardTemplate)

	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		if r.URL.Path != "/scoreboard" {
			http.NotFound(w, r)
			return
		}
		data := am.getSiteInfo(w, r)
		if data.IsSecretEvent {
			_, err := am.getTeamFromRequest(w, r)
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
			}
		}
		if err := tmpl.Execute(w, data); err != nil {
			log.Println("template err index: ", err)
		}
	}
}

func (am *Amigo) handleFlagVerify(stopExercise func(t *store.Team, challengeTag string, state bool) error) http.HandlerFunc {
	type verifyFlagMsg struct {
		Tag  string `json:"tag"`
		Flag string `json:"flag"`
	}

	type replyMsg struct {
		Status string `json:"status"`
	}

	endpoint := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
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

		flag := strings.TrimSpace(msg.Flag)
		parentTag := getParentChallengeTag(msg.Tag)
		tag := store.Tag(msg.Tag)
		if err := team.VerifyFlag(store.Challenge{Tag: tag}, flag); err != nil {
			replyJson(http.StatusOK, w, errReply{err.Error()})
			return
		}

		replyJson(http.StatusOK, w, replyMsg{"ok"})
		// recaptcha secret is added for tests
		if am.recaptcha.secret != "" {
			go func() {
				childrenChals := team.GetChildChallenges(parentTag)
				var solvedChildChals []string
				for _, ch := range childrenChals {
					solvedTime := team.IsTeamSolvedChallenge(ch)
					if solvedTime != nil {
						solvedChildChals = append(solvedChildChals, ch)
					}
				}
				if len(solvedChildChals) == len(childrenChals) {
					if err := stopExercise(team, parentTag, false); err != nil {
						log.Print("Stop exercise failed for solved challenges")
					}
					team.AddDisabledChal(parentTag)
				}
			}()
		}
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
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
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
	signupTemplate := wd + "/svcs/amigo/resources/private/signup.tmpl.html"
	tmpl, err := parseTemplates(signupTemplate)
	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		if err := tmpl.Execute(w, am.getSiteInfo(w, r)); err != nil {
			log.Println("template err signup: ", err)
		}
	}
}

func (am *Amigo) handleSignupPOST(hook func(t *store.Team) error) http.HandlerFunc {
	signupTemplate := wd + "/svcs/amigo/resources/private/signup.tmpl.html"
	tmpl, err := parseTemplates(signupTemplate)
	if err != nil {
		log.Println("error index tmpl: ", err)
	}

	type signupData struct {
		Email          string
		TeamName       string
		Password       string
		SignupError    string
		SecretEventKey string
	}

	readParams := func(r *http.Request) (signupData, error) {
		data := signupData{
			Email:          "", // removed due to GDPR
			TeamName:       strings.TrimSpace(r.PostFormValue("team-name")),
			Password:       r.PostFormValue("password"),
			SecretEventKey: r.PostFormValue("secret-event-key"),
		}

		if err := checkTeamName(data.TeamName); err != nil {
			return data, err
		}

		if len(data.Password) < passMinLength {
			return data, fmt.Errorf("Password needs to be at least %d characters", passMinLength)
		}

		if len(data.Password) > passMaxLength {
			return data, fmt.Errorf("The maximum password length is %d characters", passMaxLength)
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
		userSecretInput := strings.TrimSpace(params.SecretEventKey)
		siteInfo := am.getSiteInfo(w, r)
		eventSecretKey := strings.TrimSpace(siteInfo.EventSecret)

		if siteInfo.EventSecret != "" {
			if userSecretInput != eventSecretKey {
				displayErr(w, params, ErrProtectedEvent)
				return
			}
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
		disabledChals := store.CopyMap(am.TeamStore.DisabledChallenges)
		allChals := store.CopyMap(am.TeamStore.AllChallenges)

		// email removed  due to GDPR
		t := store.NewTeam("", strings.TrimSpace(params.TeamName), params.Password,
			"", "", "", time.Now().UTC(),
			disabledChals, allChals, nil)

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
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
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
		if !team.IsLabAssigned() {
			replyJsonRequestErr(w, fmt.Errorf("Lab is NOT assigned to team [ %s ] on event [ %s ]", team.Name(), am.TeamStore.Tag))
			return
		}
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

func (am *Amigo) handleStartStopChallenge(runHook func(t *store.Team, challengeTag string, state bool) error) http.HandlerFunc {

	type runChallenge struct {
		Tag string `json:"tag"`
	}

	type replyMsg struct {
		Status string `json:"status"`
	}

	endpoint := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		t, err := am.getTeamFromRequest(w, r)
		if err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		var msg runChallenge
		if err := safeReadJson(w, r, &msg, am.maxReadBytes); err != nil {
			replyJsonRequestErr(w, err)
			return
		}

		chalTag := getParentChallengeTag(msg.Tag)
		if !t.IsLabAssigned() {
			replyJsonRequestErr(w, fmt.Errorf("Lab is NOT assigned to team [ %s ] on event [ %s ]", t.Name(), am.TeamStore.Tag))
			return
		}
		stopped := t.ManageDisabledChals(chalTag)
		err = runHook(t, chalTag, stopped)
		if err != nil {
			t.AddDisabledChal(chalTag)
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
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
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

func (am *Amigo) handleLogin(resumeLabHook func(t *store.Team) error) http.HandlerFunc {
	get := am.handleLoginGET()
	post := am.handleLoginPOST(resumeLabHook)

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
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

func (am *Amigo) handleLabInfo() http.HandlerFunc {
	type labInfo struct {
		IsVPN     int32  `json:"isVPN"`
		LabSubnet string `json:"labSubnet"`
	}
	endpoint := func(w http.ResponseWriter, r *http.Request) {

		if am.TeamStore.OnlyVPN == 0 {
			replyJson(http.StatusOK, w, labInfo{IsVPN: 0, LabSubnet: "VPN is not enabled !"})
		} else {
			team, err := am.getTeamFromRequest(w, r)
			if err != nil {
				replyJsonRequestErr(w, err)
				return
			}
			teamLabSubnet := team.GetLabInfo()
			tLabInfo := labInfo{
				LabSubnet: teamLabSubnet,
				IsVPN:     am.TeamStore.OnlyVPN,
			}
			replyJson(http.StatusOK, w, tLabInfo)
		}
	}
	for _, mw := range []Middleware{JSONEndpoint, POSTEndpoint} {
		endpoint = mw(endpoint)
	}
	return endpoint
}

func (am *Amigo) handleLoginGET() http.HandlerFunc {
	loginTemplate := wd + "/svcs/amigo/resources/private/login.tmpl.html"
	tmpl, err := parseTemplates(loginTemplate)
	if err != nil {
		log.Println("error login tmpl: ", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if err := tmpl.Execute(w, am.globalInfo); err != nil {
			log.Println("template err login: ", err)
		}
	}
}

func (am *Amigo) handleLoginPOST(resumeLabHook func(t *store.Team) error) http.HandlerFunc {
	loginTemplate := wd + "/svcs/amigo/resources/private/login.tmpl.html"
	tmpl, err := parseTemplates(loginTemplate)
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
		if err := resumeLabHook(t); err != nil {
			logger.Error().Str("Team id: ", t.ID()).
				Str("Team name: ", t.Name()).
				Str("Team email:", t.Email()).
				Msgf("Error on resuming team resource %v", err)
		}
	}
}

func (am *Amigo) handleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: -1})
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (am *Amigo) loginTeam(w http.ResponseWriter, r *http.Request, t *store.Team) error {
	token, err := store.GetTokenForTeam(am.signingKey, t)
	if err != nil {
		return err
	}
	w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	http.SetCookie(w, &http.Cookie{Name: "session", Value: token, MaxAge: am.cookieTTL})
	http.Redirect(w, r, "/", http.StatusSeeOther)

	//Set teams last access time
	err = t.UpdateTeamAccessed(time.Now())
	if err != nil {
		logger.Warn().
			Err(err).
			Str("team-id", t.ID()).
			Msg("Failed to update team accessed time")
	}

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

//Read json from request
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

//Return json format
func replyJson(sc int, w http.ResponseWriter, i interface{}) error {
	w.Header().Add("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(sc)

	return json.NewEncoder(w).Encode(i)
}

type errReply struct {
	Error string `json:"error"`
}

//Return json error
func replyJsonRequestErr(w http.ResponseWriter, err error) error {
	return replyJson(http.StatusBadRequest, w, errReply{err.Error()})
}

//Return team information (id and name) from cookie token
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

//Check if the request method is GET
func GETEndpoint(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

//Check if the request method is POST
func POSTEndpoint(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

//Check if the content-type of the request is in json
func JSONEndpoint(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			http.NotFound(w, r)
			return
		}
		next(w, r)
	}
}

//Get the parent tag of the challenge.
//in the exerise.yml file children challenges have a number
//for example Parent: sql, Children: sql-1, sql-2 ....
//In order to reset a challenge the parent tag is needed so, this
//function check is the challenge tag contains a number, if so, remove the last 2 characters
func getParentChallengeTag(child string) string {
	for _, c := range child {
		if unicode.IsDigit(c) {
			return child[:len(child)-2]
		}
	}
	return child
}

//Get working directory of the project
func GetWd() string {
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}
	return path
}

func parseTemplates(givenTemplate string) (*template.Template, error) {
	var tmpl *template.Template
	var err error
	tmpl, err = template.ParseFiles(
		wd+"/svcs/amigo/resources/private/base.tmpl.html",
		wd+"/svcs/amigo/resources/private/navbar.tmpl.html",
		wd+"/svcs/amigo/resources/private/notification.tmpl.html",
		givenTemplate,
	)
	return tmpl, err
}

func checkTeamName(input string) error {
	re := regexp.MustCompile(teamNameRegex)

	if input == "" {
		return ErrTeamNameEmpty
	}

	if err := checkVarLength(input, teamNameMaxLength); err != nil {
		return ErrTeamNameToLarge
	}

	if !re.MatchString(input) {
		return ErrTeamNameCharacters
	}

	return nil
}

func checkVarLength(input string, max int) error {
	if len(input) >= max {
		return fmt.Errorf("exceeds character limit")
	}
	return nil
}
