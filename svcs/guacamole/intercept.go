// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package guacamole

import (
	"errors"
	"net/http"
	"sync"

	"github.com/aau-network-security/haaukins/store"

	"github.com/aau-network-security/haaukins/svcs/amigo"
	"github.com/rs/zerolog/log"
)

const waitingHTMLTemplate = `
<html lang="en" dir="ltr">
		  <meta http-equiv="refresh" content="10" />
		  <head>
			<style>
				html, body {
		  height: 100%;
		  width: 100%;
		  margin: 0;
		  padding: 0;
		  font-size: 100%;
		  background: #191a1a;
		  text-align: center;
		}
		
		h1 {
		  margin: 100px;
		  padding: 0;
		  font-family: ‘Arial Narrow’, sans-serif;
		  font-weight: 100;
		  font-size: 1.1em;
		  color: #a3e1f0;
		}
		h2 {
		  margin:50px;
		  color: #a3e1f0;
		  font-family: ‘Arial Narrow’, sans-serif;
		}
		
		span {
		  position: relative;
		  top: 0.63em;  
		  display: inline-block;
		  text-transform: uppercase;  
		  opacity: 0;
		  transform: rotateX(-90deg);
		}
		
		.let1 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.2s;
		}
		
		.let2 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.3s;
		}
		
		.let3 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.4s;
		}
		
		.let4 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.5s;
		
		}
		
		.let5 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.6s;
		}
		
		.let6 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.7s;
		}
		
		.let7 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.8s;
		}
		
		@keyframes drop {
			10% {
				opacity: 0.5;
			}
			20% {
				opacity: 1;
				top: 3.78em;
				transform: rotateX(-360deg);
			}
			80% {
				opacity: 1;
				top: 3.78em;
				transform: rotateX(-360deg);
			}
			90% {
				opacity: 0.5;
			}
			100% {
				opacity: 0;
				top: 6.94em
			}
		}
    </style>
  </head>
  <body>
  <h1>
    <span class="let1">l</span>  
    <span class="let2">o</span>  
    <span class="let3">a</span>  
    <span class="let4">d</span>  
    <span class="let5">i</span>  
    <span class="let6">n</span>  
    <span class="let7">g</span>  
  </h1>
<h2>
Virtualized Environment
</h2>
  </body>
</html>
`

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
	us.m.RLock()
	defer us.m.RUnlock()
	us.teams[tid] = u
	log.Debug().Msgf("CreateUserForTeam function teamid: %s user guide : %s", tid, u.Username)
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
	teamStore store.Event
	amigo     *amigo.Amigo
}

func NewGuacTokenLoginEndpoint(users *GuacUserStore, ts store.Event, am *amigo.Amigo, loginFunc func(string, string) (string, error)) *guacTokenLoginEndpoint {
	return &guacTokenLoginEndpoint{
		teamStore: ts,
		users:     users,
		loginFunc: loginFunc,
		amigo:     am,
	}
}

func (*guacTokenLoginEndpoint) ValidRequest(r *http.Request) bool {
	if r.URL.Path == "/guaclogin" && r.Method == http.MethodGet {
		return true
	}

	return false
}

func (gtl *guacTokenLoginEndpoint) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(

		func(w http.ResponseWriter, r *http.Request) {
			c, err := r.Cookie("session")
			if err != nil {
				log.Debug().Msgf("Error session is not found in guacTokenLoginEndpoint %s , error is %s ", c.Value, err)
				return
			}

			session := c.Value
			t, err := gtl.amigo.TeamStore.GetTeamByToken(session)
			if err != nil {
				log.Warn().
					Err(err).
					Msg("Unable to find team by token")
				/* Write error to user */
				reportHttpError(w, "Unable to connect to lab teamStore.GetTeamByToken: ", err)
				return
			}
			u, err := gtl.users.GetUserForTeam(t.ID())
			if err != nil {
				log.Warn().
					Err(err).
					Str("team-id ", t.ID()).
					Msg("Unable to get guac user for team")
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write([]byte(waitingHTMLTemplate))
				return
			}

			token, err := gtl.loginFunc(u.Username, u.Password)
			if err != nil {
				log.Warn().
					Err(err).
					Str("team-id", t.ID()).
					Msg("Failed to login team to guacamole")
				reportHttpError(w, "Unable to connect to lab: ", err)
				return
			}

			authC := http.Cookie{Name: "GUAC_AUTH", Value: token, Path: "/guacamole/"}
			http.SetCookie(w, &authC)
			http.Redirect(w, r, "/guacamole", http.StatusFound)

		})
}

func reportHttpError(w http.ResponseWriter, msg string, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(msg))
	w.Write([]byte(err.Error()))
}
