package ws

import (
	"sync"

	"github.com/tobenna/together/server/internal/models"
)

type Hub struct {
	mu    sync.Mutex
	rooms map[string]*RoomHub
}

func NewHub() *Hub {
	return &Hub{rooms: make(map[string]*RoomHub)}
}

func (h *Hub) GetOrCreate(roomID string) *RoomHub {
	h.mu.Lock()
	defer h.mu.Unlock()
	rh, ok := h.rooms[roomID]
	if !ok {
		rh = newRoomHub(roomID)
		h.rooms[roomID] = rh
	}
	return rh
}

func (h *Hub) Get(roomID string) (*RoomHub, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	rh, ok := h.rooms[roomID]
	return rh, ok
}

func (h *Hub) BroadcastToRoom(roomID string, e Event) {
	if rh, ok := h.Get(roomID); ok {
		rh.Broadcast(e)
	}
}

func (h *Hub) SetParticipantRole(roomID, participantID string, role models.ParticipantRole) {
	if rh, ok := h.Get(roomID); ok {
		rh.SetParticipantRole(participantID, role)
	}
}
