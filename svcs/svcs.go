package svcs

import (
	"net/http"

	"github.com/aau-network-security/go-ntp/store"
	"net/http/httputil"
	"net/url"
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

func NewTlsStripReverseProxy(target string) httputil.ReverseProxy {
	var isSecure bool
	return httputil.ReverseProxy{Director: func(req *http.Request) {
		isSecure = req.TLS != nil
		req.URL.Scheme = "http"
		req.URL.Host = target
	}, ModifyResponse: func(resp *http.Response) error {
		location := resp.Header.Get("location")
		if location != "" {
			u, _ := url.Parse(location)
			scheme := "http"
			if isSecure {
				scheme = "https"
			}
			u.Scheme = scheme
			resp.Header.Set("location", u.String())
		}
		return nil
	}}
}
