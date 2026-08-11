package api

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/tobenna/together/server/internal/auth"
	"github.com/tobenna/together/server/internal/rooms"
	"github.com/tobenna/together/server/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool { return true },
}
func (s *Server) handleRoomWebSocket(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	hub := s.Hub.GetOrCreate(claims.RoomID)
	c := ws.NewConn(conn, hub, claims.ParticipantID, claims.RoomID, claims.Role)

	c.OnClose = func(c *ws.Conn) {
		hub.Unregister(c)
		if err := s.Rooms.LeaveRoom(r.Context(), c.ParticipantID); err != nil {
			log.Printf("ws: mark disconnected failed: %v", err)
		}
		hub.Broadcast(ws.NewEvent(ws.EventParticipantLeft, c.RoomID, map[string]string{
			"participantId": c.ParticipantID,
		}))
	}

	c.OnEvent = func(c *ws.Conn, e ws.Event) {
		switch e.Type {
		case ws.EventAnnotationCreated, ws.EventAnnotationUpdated, ws.EventAnnotationDeleted:
			if rooms.Can(c.Role(), rooms.ActionAnnotate) {
				hub.Broadcast(e)
			}
		case ws.EventReactionSent:
			if rooms.Can(c.Role(), rooms.ActionReact) {
				hub.Broadcast(e)
			}
		case ws.EventWebRTCOffer, ws.EventWebRTCAnswer, ws.EventWebRTCICE:
			hub.SendTo(e)
		default:
		
		}
	}

	hub.Register(c)
	c.Start()
	c.Send(ws.NewEvent(ws.EventPeersAnnounce, claims.RoomID, hub.ParticipantIDs()))
}
