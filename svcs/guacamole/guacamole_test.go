package guacamole

import (
	"testing"
	"net/http/httptest"
		"net/http"
		"github.com/gorilla/websocket"
	"strings"
)

func echo(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			break
		}
		err = c.WriteMessage(mt, message)
		if err != nil {
			break
		}
	}
}

func TestWebSocketProxy(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(echo))
	defer backend.Close()

	h := websocketProxy(strings.TrimPrefix(backend.URL, "http://"))
	wsProxy := httptest.NewServer(h)
	defer wsProxy.Close()

	ws, _, err := websocket.DefaultDialer.Dial("ws" + strings.TrimPrefix(wsProxy.URL, "http"), nil)
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer ws.Close()

	if err := ws.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("%v", err)
	}
	_, p, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if string(p) != "hello" {
		t.Fatalf("bad message")
	}
}