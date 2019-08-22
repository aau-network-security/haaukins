// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package guacamole

import (
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/aau-network-security/haaukins/store"
	"github.com/rs/zerolog/log"
)

var (
	UnknownTeamIdErr = errors.New("Unknown team id")
)

type GuacUser struct {
	Username string
	Password string
}

type GuacUserStore struct {
	m     sync.RWMutex
	teams map[string]GuacUser
}

func NewGuacUserStore() *GuacUserStore {
	return &GuacUserStore{
		teams: map[string]GuacUser{},
	}
}

func (us *GuacUserStore) CreateUserForTeam(tid string, u GuacUser) {
	us.m.Lock()
	defer us.m.Unlock()
	us.teams[tid] = u
}

func (us *GuacUserStore) GetUserForTeam(tid string) (*GuacUser, error) {
	us.m.RLock()
	defer us.m.RUnlock()

	u, ok := us.teams[tid]
	if !ok {
		return nil, UnknownTeamIdErr
	}

	return &u, nil
}

type guacTokenLoginEndpoint struct {
	users     *GuacUserStore
	loginFunc func(string, string) (string, error)
	teamStore store.TeamStore
}

func NewGuacTokenLoginEndpoint(users *GuacUserStore, ts store.TeamStore, loginFunc func(string, string) (string, error)) *guacTokenLoginEndpoint {
	return &guacTokenLoginEndpoint{
		teamStore: ts,
		users:     users,
		loginFunc: loginFunc,
	}
}

func (*guacTokenLoginEndpoint) ValidRequest(r *http.Request) bool {
	if r.URL.Path == "/guaclogin" && r.Method == http.MethodGet {
		return true
	}

	return false
}

func (gtl *guacTokenLoginEndpoint) Intercept(next http.Handler) http.Handler {
	httpErr := func(w http.ResponseWriter, err error) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session")
		if err != nil {
			return
		}

		session := c.Value
		t, err := gtl.teamStore.GetTeamByToken(session)
		if err != nil {
			log.Warn().
				Err(err).
				Msg("Unable to find team by token")
			httpErr(w, fmt.Errorf("Unable to find team: %v", err))
			return
		}
		u, err := gtl.users.GetUserForTeam(t.Id)
		if err != nil {
			httpErr(w, fmt.Errorf("Unable to find team: %s", err))
			return
		}

		token, err := gtl.loginFunc(u.Username, u.Password)
		if err != nil {
			httpErr(w, fmt.Errorf("Unable to log into guacamole to lab: %s", err))
			return
		}

		authC := http.Cookie{Name: "GUAC_AUTH", Value: token, Path: "/guacamole/"}
		http.SetCookie(w, &authC)
		http.Redirect(w, r, "/guacamole", http.StatusFound)
	})
}
