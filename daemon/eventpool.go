// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package daemon

import (
	"net/http"
	"strings"
	"sync"

	wg "github.com/aau-network-security/haaukins/network/vpn"

	"github.com/aau-network-security/haaukins/svcs/guacamole"

	"github.com/aau-network-security/haaukins/store"
)

type eventPool struct {
	m               sync.RWMutex
	host            string
	notFoundHandler http.Handler
	events          map[store.Tag]guacamole.Event
	handlers        map[store.Tag]http.Handler
	wg              wg.WireguardClient
}

func NewEventPool(host string) *eventPool {
	return &eventPool{
		host:            host,
		notFoundHandler: notFoundHandler(),
		events:          map[store.Tag]guacamole.Event{},
		handlers:        map[store.Tag]http.Handler{},
	}
}

func (ep *eventPool) AddEvent(ev guacamole.Event) {
	tag := ev.GetConfig().Tag

	ep.m.Lock()
	defer ep.m.Unlock()

	ep.events[tag] = ev
	ep.handlers[tag] = ev.Handler()
}

func (ep *eventPool) RemoveEvent(t store.Tag) error {
	ep.m.Lock()
	defer ep.m.Unlock()

	if _, ok := ep.events[t]; !ok {
		return UnknownEventErr
	}

	delete(ep.events, t)
	delete(ep.handlers, t)

	return nil
}

func (ep *eventPool) GetEvent(t store.Tag) (guacamole.Event, error) {
	ep.m.RLock()
	ev, ok := ep.events[t]
	ep.m.RUnlock()
	if !ok {
		return nil, UnknownEventErr
	}

	return ev, nil
}

func (ep *eventPool) GetAllEvents() []guacamole.Event {
	events := make([]guacamole.Event, len(ep.events))

	var i int
	ep.m.RLock()
	for _, ev := range ep.events {
		events[i] = ev
		i++
	}
	ep.m.RUnlock()

	return events
}

func (ep *eventPool) Close() error {
	var firstErr error

	for _, ev := range ep.events {
		if err := ev.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (ep *eventPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	domainParts := strings.SplitN(r.Host, ".", 2)

	if len(domainParts) != 2 {
		ep.notFoundHandler.ServeHTTP(w, r)
		return
	}

	sub, dom := domainParts[0], domainParts[1]
	if !strings.HasPrefix(dom, ep.host) {
		ep.notFoundHandler.ServeHTTP(w, r)
		return
	}

	ep.m.RLock()
	mux, ok := ep.handlers[store.Tag(sub)]
	ep.m.RUnlock()
	if !ok {
		ep.notFoundHandler.ServeHTTP(w, r)
		return
	}

	mux.ServeHTTP(w, r)
}

func getHost(r *http.Request) string {
	if r.URL.IsAbs() {
		host := r.Host
		// Slice off any port information.
		if i := strings.Index(host, ":"); i != -1 {
			host = host[:i]
		}
		return host
	}
	return r.URL.Host

}

func notFoundHandler() http.Handler {
	p := []byte(notfoundpage)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write(p)
	})
}

func suspendEventHandler() http.Handler {
	p := []byte(suspendPage)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTemporaryRedirect)
		w.Write(p)
	})
}
