package amigo

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/aau-network-security/haaukins"
	"github.com/aau-network-security/haaukins/store"
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

type SBClient struct {
	teamId string
	sb     *Scoreboard
	conn   *websocket.Conn
	send   chan []byte
}

func (c *SBClient) writePump() {
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

// serveWs handles websocket requests from the peer.
func serveWs(sb *Scoreboard, w http.ResponseWriter, r *http.Request) {
}

type Scoreboard struct {
	ts         store.TeamStore
	challenges []haaukins.Challenge
	chalMsg    []byte
	clients    map[*SBClient]struct{}
	register   chan *SBClient
	unregister chan *SBClient
}

func newScoreboard(ts store.TeamStore, chals ...haaukins.Challenge) *Scoreboard {
	msg := SBMessage{
		Message: "challenges",
		Values:  chals,
	}
	chalMsg, _ := json.Marshal(msg)

	return &Scoreboard{
		ts:         ts,
		challenges: chals,
		chalMsg:    chalMsg,
		register:   make(chan *SBClient),
		unregister: make(chan *SBClient),
		clients:    make(map[*SBClient]struct{}),
	}
}

type TeamRow struct {
	Id          string       `json:"id"`
	Name        string       `json:"name"`
	Completions []*time.Time `json:"completions"`
	IsUser      bool         `json:"is_user"`
}

func TeamRowFromTeam(t *haaukins.Team) TeamRow {
	chals := t.GetChallenges()
	completions := make([]*time.Time, len(chals))
	for i, chal := range chals {
		completions[i] = chal.CompletedAt
	}

	return TeamRow{
		Id:          t.ID(),
		Name:        t.Name(),
		Completions: completions,
	}
}

func (sb *Scoreboard) initChallenges() []byte {
	return sb.chalMsg
}

func (sb *Scoreboard) initTeams(teamId string) []byte {
	teams := sb.ts.GetTeams()
	rows := make([]TeamRow, len(teams))
	for i, t := range teams {
		r := TeamRowFromTeam(t)
		if t.ID() == teamId {
			r.IsUser = true
		}
		rows[i] = r
	}

	msg := SBMessage{
		Message: "teams",
		Values:  rows,
	}
	rawMsg, _ := json.Marshal(msg)

	return rawMsg
}

type SBMessage struct {
	Message string      `json:"msg"`
	Values  interface{} `json:"values"`
}

func (sb *Scoreboard) run() {
	for {
		select {
		case client := <-sb.register:
			select {
			case client.send <- sb.initChallenges():
			default:
				continue
			}

			select {
			case client.send <- sb.initTeams(client.teamId):
			default:
				continue
			}

			sb.clients[client] = struct{}{}
		case client := <-sb.unregister:
			if _, ok := sb.clients[client]; ok {
				delete(sb.clients, client)
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

func (sb *Scoreboard) handleConns() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		client := &SBClient{sb: sb, conn: conn, send: make(chan []byte, 256)}
		client.sb.register <- client

		go client.writePump()

	}
}
