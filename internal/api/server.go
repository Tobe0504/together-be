package api

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/tobenna/together/server/internal/auth"
	"github.com/tobenna/together/server/internal/config"
	"github.com/tobenna/together/server/internal/db"
	"github.com/tobenna/together/server/internal/discovery"
	"github.com/tobenna/together/server/internal/livekit"
	"github.com/tobenna/together/server/internal/models"
	"github.com/tobenna/together/server/internal/rooms"
	"github.com/tobenna/together/server/internal/ws"
)

type Server struct {
	Cfg     config.Config
	Store   *db.Store
	Signer  *auth.Signer
	Rooms   *rooms.Service
	Hub     *ws.Hub
	LiveKit *livekit.TokenIssuer
}

func NewServer(cfg config.Config, store *db.Store) *Server {
	signer := auth.NewSigner(cfg.JWTSecret)
	return &Server{
		Cfg:     cfg,
		Store:   store,
		Signer:  signer,
		Rooms:   rooms.NewService(store, signer),
		Hub:     ws.NewHub(),
		LiveKit: livekit.NewTokenIssuer(cfg.LiveKitAPIKey, cfg.LiveKitAPISecret, cfg.LiveKitURL),
	}
}

func (s *Server) syncRoomRoles(ctx context.Context, roomID string) {
	participants, err := s.Rooms.ListParticipants(ctx, roomID)
	if err != nil {
		log.Printf("sync roles for room %s: %v", roomID, err)
		return
	}
	for _, p := range participants {
		s.Hub.SetParticipantRole(roomID, p.ID, p.Role)
	}

	room, err := s.Rooms.GetRoom(ctx, roomID)
	if err != nil || room.Mode != models.RoomModeOnline {
		return
	}

	snapshot := make([]struct {
		ID   string
		Role models.ParticipantRole
	}, 0, len(participants))
	for _, p := range participants {
		snapshot = append(snapshot, struct {
			ID   string
			Role models.ParticipantRole
		}{p.ID, p.Role})
	}
	go func() {
		bg, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, p := range snapshot {
			err := s.LiveKit.UpdatePermissions(bg, roomID, p.ID, p.Role)
			if err != nil && !errors.Is(err, livekit.ErrParticipantNotConnected) {
				log.Printf("livekit permission sync (room %s, participant %s): %v", roomID, p.ID, err)
			}
		}
	}()
}

func (s *Server) LANAddrHint() string {
	ip, err := discovery.PrimaryLANIP()
	if err != nil || ip == "" {
		return ""
	}
	return ip + ":" + s.Cfg.Port
}
