package api

import (
	"errors"
	"fmt"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
	"strings"
)

const (
	STATUS			= "STATUS"
	CHALLENGES_KEY	= "CH"
	token_key		= "test"		//todo its just for testing purpose
)

var (
	UnknownTokenErr     = errors.New("Unknown token")
	ErrorCreateToken	= errors.New("Unable to create the Token")
)

type LearningMaterialAPI struct {
	crs 		ClientRequestStore
	maxRequests	int
	exStore     store.ExerciseStore
	vlib  		vbox.Library
	frontend 	[]store.InstanceConfig
}

func NewLearningMaterialAPI() (*LearningMaterialAPI, error) {
	// better approach is to read from a configuration file
	crs:= NewClientRequestStore()
	vlib := vbox.NewLibrary("/home/gian/Documents/ova")
	frontends :=  []store.InstanceConfig{{
		Image: "kali",
		MemoryMB: uint(4096),
	}}
	ef, err := store.NewExerciseFile("/home/gian/Documents/haaukins_files/configs/exercises.yml")
	if err != nil {
		return nil, err
	}
	return &LearningMaterialAPI{
		crs:         *crs,
		maxRequests: 4,
		exStore:     ef,
		vlib:        vlib,
		frontend:    frontends,
	}, nil
}

func (lm *LearningMaterialAPI) Handler() http.Handler {
	m := mux.NewRouter()
	m.HandleFunc("/api/{chals}", lm.RequestFromLearningMaterial())
	return m
}

func (lm *LearningMaterialAPI) RequestFromLearningMaterial() http.HandlerFunc{
	var errs error

	return func(w http.ResponseWriter, r *http.Request) {
		lm.crs.m.RLock()
		defer lm.crs.m.RUnlock()

		challengesFromLink := mux.Vars(r)["chals"]
		challenges := strings.Split(challengesFromLink, ",")
		challengesTag, err := lm.GetChallengesFromRequest(challenges)

		//Bad request (challenge tags don't exist)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(errorHTMLTemplate))
			return
		}

		//check if the user exists
		clientR, ok := lm.crs.clientsR[r.Host]

		//The user is new so a newEnvironment and a NewClientRequest will be created
		if !ok {
			if err := lm.NewClientNewRequest(r, challengesTag); err != nil{
				errs = err
			}
			//cookie used to avoid multiple Environment is created when the page is refreshed
			token, _ := CreateTokenForClientByChallenge(token_key, challengesFromLink)
			http.SetCookie(w, &http.Cookie{Name: "session", Value: token})

			w.WriteHeader(http.StatusServiceUnavailable)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(waitingHTMLTemplate))
			return
		}

		//check if the challenges the user requested exist
		cookie, ok := clientR.cookies[challengesFromLink]

		//The user already exists so a newEnvironment will be created
		if !ok {
			if err := lm.KnownClientNewRequest(r, clientR, challengesTag); err != nil{
				errs = err
			}
			token, _ := CreateTokenForClientByChallenge(token_key, challengesFromLink)
			http.SetCookie(w, &http.Cookie{Name: "session", Value: token})

			w.WriteHeader(http.StatusServiceUnavailable)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(waitingHTMLTemplate))
			return
		}

		port, _ := clientR.ports[challengesFromLink]

		authC := http.Cookie{Name: "GUAC_AUTH", Value: cookie , Path: "/guacamole/"}
		http.SetCookie(w, &authC)
		http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: -1}) //Remove the cookie
		host := fmt.Sprintf("http://localhost:%s/guacamole", strconv.Itoa(int(port)))
		http.Redirect(w, r, host, http.StatusFound)
	}
}

//Get the challenges from the store, return error if the challenges tag dosen't exist
func (lm *LearningMaterialAPI) GetChallengesFromRequest(challenges []string) ([]store.Tag, error){

	tags := make([]store.Tag, len(challenges))
	for i, s := range challenges {
		t := store.Tag(s)
		_, tagErr := lm.exStore.GetExercisesByTags(t)
		if tagErr != nil {
			return nil, tagErr
		}
		tags[i] = t
	}
	return tags, nil
}

//function called when the client is new from ClientRequestStore
func (lm *LearningMaterialAPI) NewClientNewRequest(r *http.Request, challenges []store.Tag) error {
	var errs error
	c, _ := r.Cookie("session")
	if c != nil {	// if cookie exists return, so another environment will not be created
		return errs
	}
	go func(host string, challenges []store.Tag) {
		env, err := lm.newEnvironment(challenges)
		if err != nil {
			errs = err
			return
		}
		client := lm.crs.NewClientRequest(host)
		err = env.Assign(client)
		if err != nil {
			errs = err
			return
		}
		client.requestsMade += 1
	}(r.Host, challenges)

	return errs
}

//function called when the client already exists in the ClientRequestStore
func (lm *LearningMaterialAPI) KnownClientNewRequest(r *http.Request, client *ClientRequest, challenges []store.Tag) error {
	var errs error
	c, _ := r.Cookie("session")
	if c != nil {	// if cookie exists return, so another environment will not be created
		return errs
	}
	go func(client *ClientRequest, challenges []store.Tag) {
		env, err := lm.newEnvironment(challenges)
		if err != nil {
			errs = err
			return
		}
		err = env.Assign(client)
		if err != nil {
			errs = err
			return
		}
		client.requestsMade += 1
	}(client, challenges)

	return errs
}

//Create the token for the challenge per client
func CreateTokenForClientByChallenge(key, chals string) (string, error){
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		STATUS:	"creating",
		CHALLENGES_KEY: chals,
	})
	tokenStr, err := token.SignedString([]byte(key))
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}
