package api

import (
	"net/http"

	"github.com/tobenna/together/server/internal/auth"
	"github.com/tobenna/together/server/internal/models"
	"github.com/tobenna/together/server/internal/rooms"
	"github.com/tobenna/together/server/internal/ws"
)

type joinRoomRequest struct {
	DisplayName string `json:"displayName"`
	PIN         string `json:"pin"`
}

func (s *Server) handleJoinRoom(w http.ResponseWriter, r *http.Request) {
	var req joinRoomRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Please check your details and try again.")
		return
	}
	if req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "Tell us your name to join.")
		return
	}

	var userID *string
	if claims := auth.AccountClaimsFromContext(r.Context()); claims != nil {
		userID = &claims.UserID
	}

	result, err := s.Rooms.JoinRoom(r.Context(), rooms.JoinRoomInput{
		RoomID: chiRoomID(r), DisplayName: req.DisplayName, PIN: req.PIN, UserID: userID,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}

	s.Hub.BroadcastToRoom(result.Room.ID, ws.NewEvent(ws.EventParticipantJoined, result.Room.ID, result.Participant))

	writeJSON(w, http.StatusOK, map[string]any{
		"room": result.Room, "participant": result.Participant, "token": result.Token,
	})
}

func (s *Server) handleListParticipants(w http.ResponseWriter, r *http.Request) {
	list, err := s.Rooms.ListParticipants(r.Context(), chiRoomID(r))
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleLeaveRoom(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	if err := s.Rooms.LeaveRoom(r.Context(), claims.ParticipantID); err != nil {
		mapServiceError(w, err)
		return
	}
	s.Hub.BroadcastToRoom(claims.RoomID, ws.NewEvent(ws.EventParticipantLeft, claims.RoomID, map[string]string{
		"participantId": claims.ParticipantID,
	}))
	writeJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

func (s *Server) handleKickParticipant(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	target := chiParticipantID(r)
	if err := s.Rooms.KickParticipant(r.Context(), claims.Role, target); err != nil {
		mapServiceError(w, err)
		return
	}
	s.Hub.BroadcastToRoom(claims.RoomID, ws.NewEvent(ws.EventParticipantLeft, claims.RoomID, map[string]string{
		"participantId": target, "reason": "kicked",
	}))
	writeJSON(w, http.StatusOK, map[string]string{"status": "kicked"})
}

type patchRoleRequest struct {
	Role string `json:"role"`
}

func (s *Server) handlePatchParticipantRole(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	var req patchRoleRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Please check your details and try again.")
		return
	}
	newRole := models.ParticipantRole(req.Role)
	target := chiParticipantID(r)
	if err := s.Rooms.ChangeRole(r.Context(), claims.Role, target, newRole); err != nil {
		mapServiceError(w, err)
		return
	}
	s.syncRoomRoles(r.Context(), claims.RoomID)
	s.Hub.BroadcastToRoom(claims.RoomID, ws.NewEvent(ws.EventParticipantUpdated, claims.RoomID, map[string]string{
		"participantId": target, "role": string(newRole),
	}))
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}
