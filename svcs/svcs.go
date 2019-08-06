// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package svcs

import (
	"net/http"
<<<<<<< HEAD
=======

>>>>>>> 18402a531eaf753f1c7e963b0c1bf5fa0c1c631c
	"github.com/aau-network-security/haaukins/store"
)

type ProxyConnector func(store.EventFile) http.Handler

type Interception interface {
	ValidRequest(r *http.Request) bool
	Intercept(http.Handler) http.Handler
}

type Interceptors []Interception

func (inter Interceptors) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		for _, i := range inter {
			if i.ValidRequest(r) {
				i.Intercept(next).ServeHTTP(w, r)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
