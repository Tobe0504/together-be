package ws

import (
	"log"
	"sync"
	"time"
	"github.com/gorilla/websocket"
	"github.com/tobenna/together/server/internal/models"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	maxMsgSize = 64 * 1024 
)

type Conn struct {
	ws            *websocket.Conn
	hub           *RoomHub
	ParticipantID string
	RoomID        string
	roleMu sync.RWMutex
	role   models.ParticipantRole

	send chan []byte

	OnEvent func(c *Conn, e Event)
	OnClose func(c *Conn)
}

func NewConn(wsConn *websocket.Conn, hub *RoomHub, participantID, roomID string, role models.ParticipantRole) *Conn {
	return &Conn{
		ws: wsConn, hub: hub, ParticipantID: participantID, RoomID: roomID, role: role,
		send: make(chan []byte, 32),
	}
}

func (c *Conn) Role() models.ParticipantRole {
	c.roleMu.RLock()
	defer c.roleMu.RUnlock()
	return c.role
}

func (c *Conn) SetRole(role models.ParticipantRole) {
	c.roleMu.Lock()
	c.role = role
	c.roleMu.Unlock()
}
func (c *Conn) Start() {
	go c.writePump()
	go c.readPump()
}

func (c *Conn) Send(e Event) {
	b, err := marshalEvent(e)
	if err != nil {
		return
	}
	select {
	case c.send <- b:
	default:
		log.Printf("ws: dropping message for %s (send buffer full)", c.ParticipantID)
	}
}

func (c *Conn) readPump() {
	defer func() {
		if c.OnClose != nil {
			c.OnClose(c)
		}
		c.ws.Close()
	}()
	c.ws.SetReadLimit(maxMsgSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		e, err := unmarshalEvent(raw)
		if err != nil {
			continue
		}
		e.From = c.ParticipantID
		e.RoomID = c.RoomID
		if c.OnEvent != nil {
			c.OnEvent(c, e)
		}
	}
}

func (c *Conn) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
