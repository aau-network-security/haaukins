package api

import (
	"github.com/google/uuid"
	"sync"
)

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
