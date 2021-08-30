package amigo

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aau-network-security/haaukins/store"
	jwt "github.com/golang-jwt/jwt/v4"
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

	waitingHTMLTemplate = `
<html lang="en" dir="ltr">
		  <meta http-equiv="refresh" content="10" />
		  <head>
			<style>
				html, body {
		  height: 100%;
		  width: 100%;
		  margin: 0;
		  padding: 0;
		  font-size: 100%;
		  background: #191a1a;
		  text-align: center;
		}
		
		h1 {
		  margin: 100px;
		  padding: 0;
		  font-family: ‘Arial Narrow’, sans-serif;
		  font-weight: 100;
		  font-size: 1.1em;
		  color: #a3e1f0;
		}
		h2 {
		  margin:50px;
		  color: #a3e1f0;
		  font-family: ‘Arial Narrow’, sans-serif;
		}
		
		span {
		  position: relative;
		  top: 0.63em;  
		  display: inline-block;
		  text-transform: uppercase;  
		  opacity: 0;
		  transform: rotateX(-90deg);
		}
		
		.let1 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.2s;
		}
		
		.let2 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.3s;
		}
		
		.let3 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.4s;
		}
		
		.let4 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.5s;
		
		}
		
		.let5 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.6s;
		}
		
		.let6 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.7s;
		}
		
		.let7 {
		  animation: drop 1.2s ease-in-out infinite;
		  animation-delay: 1.8s;
		}
		
		@keyframes drop {
			10% {
				opacity: 0.5;
			}
			20% {
				opacity: 1;
				top: 3.78em;
				transform: rotateX(-360deg);
			}
			80% {
				opacity: 1;
				top: 3.78em;
				transform: rotateX(-360deg);
			}
			90% {
				opacity: 0.5;
			}
			100% {
				opacity: 0;
				top: 6.94em
			}
		}
    </style>
  </head>
  <body>
  <h1>
    <span class="let1">l</span>  
    <span class="let2">o</span>  
    <span class="let3">a</span>  
    <span class="let4">d</span>  
    <span class="let5">i</span>  
    <span class="let6">n</span>  
    <span class="let7">g</span>  
  </h1>
<h2>
Virtualized Environment
</h2>
  </body>
</html>
`
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
	challenges []store.ChildrenChalConfig
	clients    map[*Client]struct{}
	update     chan []store.ChildrenChalConfig
	register   chan *Client
	unregister chan *Client
}

func NewFrontendData(ts store.TeamStore, chals ...store.ChildrenChalConfig) *FrontendData {

	return &FrontendData{
		ts:         ts,
		challenges: chals,
		register:   make(chan *Client),
		update:     make(chan []store.ChildrenChalConfig),
		unregister: make(chan *Client),
		clients:    make(map[*Client]struct{}),
	}
}

func (fd *FrontendData) UpdateChallenges(chals []store.ChildrenChalConfig) {
	go func() {
		fd.update <- chals
	}()

}

func (fd *FrontendData) RunFrontendData() {
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
			select {
			case newChs := <-fd.update:
				fd.challenges = append(fd.challenges, newChs...)
			default:
				continue
			}
			fd.clients[client] = struct{}{}
		case client := <-fd.unregister:
			if _, ok := fd.clients[client]; ok {
				delete(fd.clients, client)
				close(client.send)
			}
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
