package amigo

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aau-network-security/haaukins/store"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type Client struct {
	teamId string
	fd     *FrontendData
	conn   *websocket.Conn
	send   chan []byte
}

type FrontendData struct {
	ts         store.TeamStore
	challenges []store.FlagConfig
	clients    map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
}

func newFrontendData(ts store.TeamStore, chals ...store.FlagConfig) *FrontendData {
	return &FrontendData{
		ts:         ts,
		challenges: chals,
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]struct{}),
	}
}

func (fd *FrontendData) runFrontendData() {
	for {
		select {
		case client := <-fd.register:
			select {
			case client.send <- fd.initChallenges(client.teamId):
			default:
				continue
			}

			select {
			case client.send <- fd.initTeams(client.teamId):
			default:
				continue
			}

			fd.clients[client] = struct{}{}
		case client := <-fd.unregister:
			if _, ok := fd.clients[client]; ok {
				delete(fd.clients, client)
				close(client.send)
			}
			// case message := <-sb.broadcast:
			// 	for client := range sb.clients {
			// 		select {
			// 		case client.send <- message:
			// 		default:
			// 			close(client.send)
			// 			delete(sb.clients, client)
			// 		}
			// 	}
		}
	}
}

func (fd *FrontendData) handleConns() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		//todo manage the isUser field
		team, err := getTeamFromRequest(w, r)
		teamId := ""
		if err == nil {
			teamId = team.Id
		}
		client := &Client{teamId: teamId, fd: fd, conn: conn, send: make(chan []byte, 256)}
		client.fd.register <- client

		go client.writePump()

	}
}

// serveWs handles websocket requests from the peer.
//func serveWs(sb *Scoreboard, w http.ResponseWriter, r *http.Request) {
//}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

//todo change this function. there are 2 same function in amigo
func getTeamInfoFromToken(token string) (*team, error) {
	jwtToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		//todo modifiy here the signing key
		return []byte(signingKey), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok || !jwtToken.Valid {
		return nil, ErrInvalidTokenFormat
	}

	id, ok := claims[ID_KEY].(string)
	if !ok {
		return nil, ErrInvalidTokenFormat
	}

	name, ok := claims[TEAMNAME_KEY].(string)
	if !ok {
		return nil, ErrInvalidTokenFormat
	}

	return &team{Id: id, Name: name}, nil
}

func getTeamFromRequest(w http.ResponseWriter, r *http.Request) (*team, error) {
	c, err := r.Cookie("session")
	if err != nil {
		return nil, ErrUnauthorized
	}

	team, err := getTeamInfoFromToken(c.Value)
	if err != nil {
		http.SetCookie(w, &http.Cookie{Name: "session", MaxAge: -1})
		return nil, err
	}

	return team, nil
}
