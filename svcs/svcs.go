package svcs

import (
	"net/http"
	"github.com/rs/zerolog/log"
)

type ProxyConnector interface {
	ProxyHandler() http.Handler
}

type Interception interface {
	ValidRequest(r *http.Request) bool
	Intercept(http.Handler) http.Handler
}

type Interceptors []Interception

func (inter Interceptors) Intercept(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for n, i := range inter {
			log.Debug().Msgf("%d: %s", n, i.ValidRequest(r))
			if i.ValidRequest(r) {
				i.Intercept(next).ServeHTTP(w, r)
				log.Debug().Msgf("Succesfully intercepted!")
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
