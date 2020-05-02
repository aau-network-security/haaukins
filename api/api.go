package api

import (
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"net/http"
	"strings"
	"sync"
	"time"
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
	cookieTTL	int
	maxRequests	int
}

func NewLearningMaterialAPI() *LearningMaterialAPI {
	crs := NewClientRequestStore()
	return &LearningMaterialAPI{
		crs:         *crs,
		cookieTTL:   int((2 * time.Hour).Seconds()),		//2 hours (time in which the environment is available)
		maxRequests: 4,
	}
}

func (lm *LearningMaterialAPI) Handler() http.Handler {
	m := mux.NewRouter()
	m.HandleFunc("/api/{chals}", lm.RequestFromLearningMaterial())
	return m
}

func (lm *LearningMaterialAPI) RequestFromLearningMaterial() http.HandlerFunc{
	return func(w http.ResponseWriter, r *http.Request) {

		challengesFromLink := mux.Vars(r)["chals"]
		challenges := strings.Split(challengesFromLink, ",")

		lm.crs.m.RLock()
		defer lm.crs.m.RUnlock()

		clientR, ok := lm.crs.clientsR[r.Host]

		if !ok { 	// the user is new
			clientR = lm.crs.NewClientRequest(r.Host)
			_, err := clientR.CreateEnvironment(challenges)
			if err != nil {
				//implement me
				//return error page
			}
			//redirect to guacamole url

		}

		//check if the challenge already exists
		// GetTokenByChallenges
		//if so take back the cookie and redirect to guacamole
		// if not create a new challenge and increase the requestsMade by 1


		fmt.Println(r.Host)
		fmt.Fprintf(w, challengesFromLink)
		//http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

type ClientRequestStore struct {
	m sync.RWMutex
	clientsR		map[string]*ClientRequest		//map with the client ip
}

func NewClientRequestStore() *ClientRequestStore {
	crs := &ClientRequestStore{
		clientsR: 	map[string]*ClientRequest{},
	}
	return crs
}

func (crs *ClientRequestStore) NewClientRequest(host string) *ClientRequest {

	cl := &ClientRequest{
		cookies:      nil,
		host:		  host,
		requestsMade: 0,
	}

	crs.clientsR[host] = cl
	return cl
}

type ClientRequest struct {
	cookies			map[string]string				//map with challenge tag
	host			string
	requestsMade 	int
}

//Get The cookie based on the challenge tag and host ip
func (cr *ClientRequest) GetTokenByChallenges(chals []string) (string, error){
	challenges := strings.Join(chals, ",")
	token, ok := cr.cookies[challenges]
	if !ok {
		return "", UnknownTokenErr
	}

	return token, nil
}

// Create the environment for the new request (run guacamole and the selected challenges)
// return the url of guacamole
func (cr *ClientRequest) CreateEnvironment(chals []string) (string, error){

	challenges := strings.Join(chals, ",")

	token, err := cr.CreateTokenForClientByChallenge(token_key, challenges)
	if err != nil {
		return "", ErrorCreateToken
	}
	//create guacamole  and the challenge

	cr.cookies[challenges] = token
	cr.requestsMade += 1

	return "url", nil
}

//Create the token for the challenge per client
func (cr *ClientRequest) CreateTokenForClientByChallenge(key, chals string) (string, error){
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		HOST_KEY:		cr.host,
		CHALLENGES_KEY: chals,
	})
	tokenStr, err := token.SignedString([]byte(key))
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}

