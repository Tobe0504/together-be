package api

import (
	"context"
	"errors"
	"log"
	"strings"
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

// LANAddrHint returns an address guests can reach this server at on the
// local network, or "" when there isn't one.
//
// Gated on --mode=local because PrimaryLANIP only knows how to find a
// private-range address on this machine — it cannot tell whether that
// address means anything to anyone else. In a hosted container it happily
// returns the container's own internal IP (10.x.x.x on Render), which then
// ended up encoded into invite QR codes that no device could ever reach.
//
// The mode flag is the operator stating "this server sits on the same
// network as its guests", which is the only place that claim can honestly
// come from. Without it, invites fall back to the page's own origin (see
// the frontend's joinUrlFor) — the public URL, which is correct for a
// deployment.
func (s *Server) LANAddrHint() string {
	if s.Cfg.Mode != "local" {
		return ""
	}
	// An explicitly configured address always wins. Inside a container
	// auto-detection can only ever find the container's own address, so
	// this is the only correct source there.
	if s.Cfg.LANAddr != "" && !strings.HasPrefix(s.Cfg.LANAddr, ":") {
		return s.Cfg.LANAddr
	}
	ip, err := discovery.PrimaryLANIP()
	if err != nil || ip == "" {
		return ""
	}
	return ip + ":" + s.Cfg.Port
}
