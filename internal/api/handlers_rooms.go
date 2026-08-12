package api

import (
	"net/http"

	"github.com/tobenna/together/server/internal/auth"
	"github.com/tobenna/together/server/internal/models"
	"github.com/tobenna/together/server/internal/rooms"
	"github.com/tobenna/together/server/internal/ws"
)

type createRoomRequest struct {
	Name            string `json:"name"`
	Mode            string `json:"mode"`
	AccessProtected bool   `json:"accessProtected"`
	PIN             string `json:"pin"`
	DisplayName     string `json:"displayName"`
}

func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var req createRoomRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Please check your details and try again.")
		return
	}
	if req.Name == "" {
		req.Name = "Untitled room"
	}
	if req.DisplayName == "" {
		req.DisplayName = "Host"
	}
	mode := models.RoomModeOnline
	if req.Mode == string(models.RoomModeLocal) {
		mode = models.RoomModeLocal
	}
	if mode != models.RoomModeOnline && mode != models.RoomModeLocal {
		writeError(w, http.StatusBadRequest, "Choose Online or Local for this room.")
		return
	}

	var ownerUserID *string
	if claims := auth.AccountClaimsFromContext(r.Context()); claims != nil {
		ownerUserID = &claims.UserID
	}

	result, err := s.Rooms.CreateRoom(r.Context(), rooms.CreateRoomInput{
		Name: req.Name, Mode: mode, AccessProtected: req.AccessProtected,
		PIN: req.PIN, OwnerUserID: ownerUserID, DisplayName: req.DisplayName,
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}

	if mode == models.RoomModeLocal {
		if addr := s.LANAddrHint(); addr != "" {
			_ = s.Store.SetRoomHostLANAddr(r.Context(), result.Room.ID, addr)
			result.Room.HostLANAddr = &addr
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"room": result.Room, "participant": result.Participant, "token": result.Token,
	})
}

func (s *Server) handleGetRoom(w http.ResponseWriter, r *http.Request) {
	room, err := s.Rooms.GetRoom(r.Context(), chiRoomID(r))
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func (s *Server) handleGetRoomByCode(w http.ResponseWriter, r *http.Request) {
	code := chiJoinCode(r)
	room, err := s.Rooms.LookupByCode(r.Context(), code)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": room.ID, "name": room.Name, "mode": room.Mode,
		"accessProtected": room.AccessProtected, "status": room.Status,
	})
}

type patchRoomRequest struct {
	Name            string `json:"name"`
	AccessProtected bool   `json:"accessProtected"`
	PIN             string `json:"pin"`
}

func (s *Server) handlePatchRoom(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	if !rooms.Can(claims.Role, rooms.ActionManageAccess) {
		writeError(w, http.StatusForbidden, "You don't have permission to do that.")
		return
	}
	var req patchRoomRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Please check your details and try again.")
		return
	}
	var pinHash *string
	if req.AccessProtected && req.PIN != "" {
		h, err := auth.HashPIN(req.PIN)
		if err != nil {
			writeInternalError(w, err)
			return
		}
		pinHash = &h
	}
	if err := s.Store.UpdateRoomAccess(r.Context(), claims.RoomID, req.Name, req.AccessProtected, pinHash); err != nil {
		writeInternalError(w, err)
		return
	}
	room, err := s.Rooms.GetRoom(r.Context(), claims.RoomID)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, room)
}

func (s *Server) handleEndRoom(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	if err := s.Rooms.EndRoom(r.Context(), claims.RoomID, claims.Role); err != nil {
		mapServiceError(w, err)
		return
	}
	s.Hub.BroadcastToRoom(claims.RoomID, ws.NewEvent(ws.EventRoomEnded, claims.RoomID, nil))
	writeJSON(w, http.StatusOK, map[string]string{"status": "ENDED"})
}

func (s *Server) handlePauseRoom(w http.ResponseWriter, r *http.Request)  { s.setPaused(w, r, true) }
func (s *Server) handleResumeRoom(w http.ResponseWriter, r *http.Request) { s.setPaused(w, r, false) }

func (s *Server) setPaused(w http.ResponseWriter, r *http.Request, paused bool) {
	claims := auth.RoomClaimsFromContext(r.Context())
	if err := s.Rooms.SetPaused(r.Context(), claims.RoomID, claims.Role, paused); err != nil {
		mapServiceError(w, err)
		return
	}
	evt := ws.EventRoomResumed
	if paused {
		evt = ws.EventRoomPaused
	}
	s.Hub.BroadcastToRoom(claims.RoomID, ws.NewEvent(evt, claims.RoomID, nil))
	writeJSON(w, http.StatusOK, map[string]bool{"paused": paused})
}

func (s *Server) handleScreenStarted(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	if err := s.Rooms.StartPresenting(r.Context(), claims.RoomID, claims.ParticipantID, claims.Role); err != nil {
		mapServiceError(w, err)
		return
	}
	s.Hub.BroadcastToRoom(claims.RoomID, ws.NewEvent(ws.EventScreenStarted, claims.RoomID, map[string]string{
		"participantId": claims.ParticipantID,
	}))
	writeJSON(w, http.StatusOK, map[string]bool{"presenting": true})
}

func (s *Server) handleShareFile(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "File sharing isn't available yet.")
}

func (s *Server) handleScreenStopped(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	if err := s.Rooms.StopPresenting(r.Context(), claims.RoomID, claims.ParticipantID); err != nil {
		mapServiceError(w, err)
		return
	}
	s.Hub.BroadcastToRoom(claims.RoomID, ws.NewEvent(ws.EventScreenStopped, claims.RoomID, map[string]string{
		"participantId": claims.ParticipantID,
	}))
	writeJSON(w, http.StatusOK, map[string]bool{"presenting": false})
}
