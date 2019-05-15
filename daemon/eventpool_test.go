// Copyright (c) 2018-2019 Aalborg University
// Use of this source code is governed by a GPLv3
// license that can be found in the LICENSE file.

package daemon

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aau-network-security/haaukins/store"
)

func TestEventPool(t *testing.T) {
	tt := []struct {
		name       string
		statusCode int
		lookup     string
		events     []string
	}{
		{name: "No subdomain", statusCode: http.StatusNotFound},
		{name: "No events", lookup: "demo", statusCode: http.StatusNotFound},
		{name: "One event", lookup: "demo", statusCode: http.StatusOK, events: []string{"demo"}},
		{name: "Multiple events", lookup: "demo", statusCode: http.StatusOK, events: []string{"demo", "not-demo"}},
		{name: "Wrong event", lookup: "demo", statusCode: http.StatusNotFound, events: []string{"not-demo"}},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ep := NewEventPool("ntp-event.dk")

			for _, tag := range tc.events {
				ep.AddEvent(&fakeEvent{conf: store.EventConfig{Tag: store.Tag(tag)}})
			}

			url := "http://ntp-event.dk"
			if tc.lookup != "" {
				url = fmt.Sprintf("http://%s.ntp-event.dk", tc.lookup)
			}

			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()
			ep.ServeHTTP(w, req)

			resp := w.Result()
			resp.Body.Close()

			if resp.StatusCode != tc.statusCode {
				t.Fatalf("expected status %d as status code, but received: %d", tc.statusCode, resp.StatusCode)
			}

		})
	}
}

func TestEventPoolRestart(t *testing.T) {
	ep := NewEventPool("ntp-event.dk")

	ev := &fakeEvent{conf: store.EventConfig{Tag: store.Tag("demo")}}

	getStatus := func() int {
		req := httptest.NewRequest("GET", "http://demo.ntp-event.dk", nil)
		w := httptest.NewRecorder()
		ep.ServeHTTP(w, req)

		resp := w.Result()
		resp.Body.Close()

		return resp.StatusCode
	}

	if s := getStatus(); s != http.StatusNotFound {
		t.Fatalf("expected status 404 as status code, but received: %d", s)
	}

	ep.AddEvent(ev)

	if s := getStatus(); s != http.StatusOK {
		t.Fatalf("expected status 200 as status code, but received: %d", s)
	}

	ep.RemoveEvent(ev.GetConfig().Tag)

	if s := getStatus(); s != http.StatusNotFound {
		t.Fatalf("expected status 404 as status code, but received: %d", s)
	}

	ep.AddEvent(ev)

	if s := getStatus(); s != http.StatusOK {
		t.Fatalf("expected status 200 as status code, but received: %d", s)
	}
}

func BenchmarkEventPoolRouting(b *testing.B) {
	ep := NewEventPool("ntp-event.dk")
	ev := &fakeEvent{conf: store.EventConfig{Tag: store.Tag("demo")}}

	ep.AddEvent(ev)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "http://demo.ntp-event.dk", nil)
		w := httptest.NewRecorder()
		ep.ServeHTTP(w, req)

		resp := w.Result()
		resp.Body.Close()
	}
}
