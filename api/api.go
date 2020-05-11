package api

import (
	"errors"
	"fmt"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
	"strings"
)

const (
	HOST_KEY		= "H"
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

		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(errorHTMLTemplate))
			return
		}

		clientR, ok := lm.crs.clientsR[r.Host]

		if !ok { 	// the user is new
			if err := lm.NewClientNewRequest(r, challengesTag); err != nil{
				errs = err
			}
			http.SetCookie(w, &http.Cookie{Name: "HAAUKINS", Value: "inprocess"})
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(waitingHTMLTemplate))
			return

		}
		cookie, ok := clientR.cookies[challengesFromLink]
		if !ok {
			fmt.Println("error getting the cookie")
		}
		port, ok := clientR.ports[challengesFromLink]
		if !ok {
			fmt.Println("error getting the port")
		}
		authC := http.Cookie{Name: "GUAC_AUTH", Value: cookie , Path: "/guacamole/"}
		http.SetCookie(w, &authC)
		host := fmt.Sprintf("http://localhost:%s/guacamole", strconv.Itoa(int(port)))
		http.Redirect(w, r, host, http.StatusFound)


	}
}

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

func (lm *LearningMaterialAPI) NewClientNewRequest(r *http.Request, challenges []store.Tag) error {
	var errs error
	go func(host string, challenges []store.Tag) {
		env, err := lm.newEnvironment(challenges)
		if err != nil {
			errs = err
		}
		client := lm.crs.NewClientRequest(host)
		err = env.Assign(client)
		if err != nil {
			errs = err
		}
	}(r.Host, challenges)

	return errs
}

////Get The cookie based on the challenge tag and host ip
//func (cr *ClientRequest) GetTokenByChallenges(chals []string) (string, error){
//	challenges := strings.Join(chals, ",")
//	token, ok := cr.cookies[challenges]
//	if !ok {
//		return "", UnknownTokenErr
//	}
//
//	return token, nil
//}
//
//
////Create the token for the challenge per client
//func (cr *ClientRequest) CreateTokenForClientByChallenge(key, chals string) (string, error){
//	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
//		HOST_KEY:		cr.host,
//		CHALLENGES_KEY: chals,
//	})
//	tokenStr, err := token.SignedString([]byte(key))
//	if err != nil {
//		return "", err
//	}
//	return tokenStr, nil
//}