package api

import (
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
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



		//challengesFromLink := mux.Vars(r)["chals"]
		//challenges := strings.Split(challengesFromLink, ",")

		lm.crs.m.RLock()
		defer lm.crs.m.RUnlock()

		clientR, ok := lm.crs.clientsR[r.Host]

		if !ok { 	// the user is new
			go func() {
				env, _ := newEnvironment([]string{"ftp"})
				client := lm.crs.NewClientRequest(r.Host)
				_ = env.Assign(client)
			}()

			w.WriteHeader(http.StatusServiceUnavailable)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(waitingHTMLTemplate))
			return

		}
		//
		cookie, ok := clientR.cookies["ftp"]
		if !ok {
			fmt.Println("error getting the cookie")
		}
		port, ok := clientR.ports["ftp"]
		if !ok {
			fmt.Println("error getting the port")
		}
		fmt.Println(cookie)
		authC := http.Cookie{Name: "GUAC_AUTH", Value: cookie , Path: "/guacamole/"}
		http.SetCookie(w, &authC)
		host := fmt.Sprintf("http://localhost:%s/guacamole", strconv.Itoa(int(port)))
		fmt.Println(host)
		http.Redirect(w, r, host, http.StatusFound)


		//http.Redirect(w, r, "http://127.0.0.1:"+string(1234)+"/guacamole", http.StatusFound)
		//
		//token, ok := clientR.cookies[challengesFromLink]
		//if !ok {	// the challenge requested is new
		//	//implement me
		//	//create new environment
		//}
		//
		//// redirect to guacamole with the token found
		//
		//
		//fmt.Println(token)


		//CreateGuacamole()
		//if err != nil {
		//	fmt.Println("daddada")
		//	authC := http.Cookie{Name: "GUAC_AUTH", Value: token, Path: "/guacamole/"}
		//	http.SetCookie(w, &authC)
		//	http.Redirect(w, r, "/guacamole", http.StatusFound)
		//}

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
		username:	  uuid.New().String(),
		password:     uuid.New().String(),
		cookies: 	  map[string]string{},
		host:		  host,
		ports:  	  map[string]uint{},
		requestsMade: 0,
	}

	crs.clientsR[host] = cl
	return cl
}

type ClientRequest struct {
	username 		string
	password 		string
	cookies			map[string]string				//map with challenge tag
	host			string
	ports 			map[string]uint
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
func (cr *ClientRequest) CreateEnvironment(chals []string) error{

	//challenges := strings.Join(chals, ",")



	//token, err := cr.CreateTokenForClientByChallenge(token_key, challenges)
	//if err != nil {
	//	return "", ErrorCreateToken
	//}
	////create guacamole  and the challenge
	//
	//cr.cookies[challenges] = token
	//cr.requestsMade += 1

	return nil
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