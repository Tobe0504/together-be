package ws

import (
	"sync"

	"github.com/tobenna/together/server/internal/models"
)

type RoomHub struct {
	roomID string

	register   chan *Conn
	unregister chan *Conn
	broadcast  chan Event
	direct     chan Event

	mu    sync.RWMutex
	conns map[string]*Conn
}

func newRoomHub(roomID string) *RoomHub {
	h := &RoomHub{
		roomID:     roomID,
		register:   make(chan *Conn),
		unregister: make(chan *Conn),
		broadcast:  make(chan Event, 64),
		direct:     make(chan Event, 64),
		conns:      make(map[string]*Conn),
	}
	go h.run()
	return h
}

func (h *RoomHub) run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.conns[c.ParticipantID] = c
			h.mu.Unlock()

		case c := <-h.unregister:
			h.mu.Lock()
			if existing, ok := h.conns[c.ParticipantID]; ok && existing == c {
				delete(h.conns, c.ParticipantID)
				close(c.send)
			}
			h.mu.Unlock()

		case e := <-h.broadcast:
			h.mu.RLock()
			for _, c := range h.conns {
				c.Send(e)
			}
			h.mu.RUnlock()

		case e := <-h.direct:
			h.mu.RLock()
			if c, ok := h.conns[e.To]; ok {
				c.Send(e)
			}
			h.mu.RUnlock()
		}
	}
}

func (h *RoomHub) Register(c *Conn)   { h.register <- c }
func (h *RoomHub) Unregister(c *Conn) { h.unregister <- c }
func (h *RoomHub) Broadcast(e Event)  { h.broadcast <- e }
func (h *RoomHub) SendTo(e Event)     { h.direct <- e }

func (h *RoomHub) SetParticipantRole(participantID string, role models.ParticipantRole) {
	h.mu.RLock()
	c, ok := h.conns[participantID]
	h.mu.RUnlock()
	if ok {
		c.SetRole(role)
	}
}

func (h *RoomHub) ParticipantIDs() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	ids := make([]string, 0, len(h.conns))
	for id := range h.conns {
		ids = append(ids, id)
	}
	return ids
}

func (h *RoomHub) IsEmpty() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns) == 0
}
