package svcs

import "net/http"

type ProxyConnector interface {
	ProxyHandler() http.Handler
}
