package api

import (
	"context"
	"errors"
	"fmt"
	"github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/guacamole"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/aau-network-security/haaukins/virtual/vbox"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
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
		//challenges := strings.Split(challengesFromLink, ",")
		//
		//lm.crs.m.RLock()
		//defer lm.crs.m.RUnlock()
		//
		//clientR, ok := lm.crs.clientsR[r.Host]
		//
		//if !ok { 	// the user is new
		//	clientR = lm.crs.NewClientRequest(r.Host)
		//	_, err := clientR.CreateEnvironment(challenges)
		//	if err != nil {
		//		//implement me
		//		//return error page
		//	}
		//	//redirect to guacamole url
		//
		//}
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


		aa, err := CreateGuacamole()
		fmt.Println(err)
		fmt.Println(aa)
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
		cookies: 	  map[string]string{},
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

func CreateGuacamole() (string, error){

	ctx := context.Background()
	ef, err := store.NewExerciseFile("/home/gian/Documents/haaukins_files/configs/exercises.yml")
	exercises := store.Tag("ftp")
	exer, err := ef.GetExercisesByTags(exercises)
	if err != nil {
		return "", err
	}

	labConf := lab.Config{
		Exercises: exer,
		Frontends: []store.InstanceConfig{{
			Image: "kali",
			MemoryMB: uint(4096),
		}},
	}

	vlib := vbox.NewLibrary("/home/gian/Documents/ova")

	lh := lab.LabHost{
		Vlib: vlib,
		Conf: labConf,
	}
	labhub, err := lab.NewHub(ctx, &lh, 1, 2)
	if err != nil {
		return "", err
	}


	guac, err := guacamole.New(ctx, guacamole.Config{})
	if err != nil {
		return "", err
	}

	if err := guac.Start(ctx); err != nil {
		return "", err
	}

	lab, ok := <-labhub.Queue()
	if !ok {
		return "", errors.New("not enough lab")
	}

	if err := AssignLab("team", lab, guac); err != nil {
		fmt.Println("Issue assigning lab: ", err)
		return "", err
	}

	return "ok", nil
}

func AssignLab(user string, lab lab.Lab, guac guacamole.Guacamole) error{
	rdpPorts := lab.RdpConnPorts()
	if n := len(rdpPorts); n == 0 {
		log.
			Debug().
			Int("amount", n).
			Msg("Too few RDP connections")

		return errors.New("RdpConfErr")
	}
	u := guacamole.GuacUser{
		Username: user,
		Password: user,
	}

	if err := guac.CreateUser(u.Username, u.Password); err != nil {
		log.
			Debug().
			Str("err", err.Error()).
			Msg("Unable to create guacamole user")
		return err
	}

	dockerHost := docker.NewHost()
	hostIp, err := dockerHost.GetDockerHostIP()
	if err != nil {
		return err
	}

	for i, port := range rdpPorts {
		num := i + 1
		name := fmt.Sprintf("%s-client%d", user, num)

		log.Debug().Str("team", user).Uint("port", port).Msg("Creating RDP Connection for group")
		if err := guac.CreateRDPConn(guacamole.CreateRDPConnOpts{
			Host:     hostIp,
			Port:     port,
			Name:     name,
			GuacUser: u.Username,
			Username: &u.Username,
			Password: &u.Password,
		}); err != nil {
			return err
		}
	}

	return nil
}