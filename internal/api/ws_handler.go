package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tobenna/together/server/internal/auth"
	"github.com/tobenna/together/server/internal/rooms"
	"github.com/tobenna/together/server/internal/ws"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
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
		// NOT r.Context(): that belongs to the HTTP request, which is
		// finished the moment the connection is upgraded — so every
		// disconnect failed with "context canceled" and the participant
		// stayed CONNECTED forever. That's the ghost that lingers in
		// everyone's participant list after someone refreshes, and it also
		// silently consumed a slot against the local-room capacity limit.
		// A close can arrive at any time, long after the request is gone,
		// so it needs a context of its own.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Rooms.LeaveRoom(ctx, c.ParticipantID); err != nil {
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
