package svcs

import "net/http"

type ProxyConnector interface {
	ProxyHandler() http.Handler
}

type Interception interface {
	ValidRequest(func(r *http.Request)) bool
	Intercept(http.Handler) http.Handler
}

type Interceptors []Interception

func (i Interceptors) Intercept(http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	})
}
