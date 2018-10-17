package svcs

import (
	"net/http"
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
		for _, i := range inter {
			if i.ValidRequest(r) {
				i.Intercept(next).ServeHTTP(w, r)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
