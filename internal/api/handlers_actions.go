package api

import (
	"net/http"

	"github.com/tobenna/together/server/internal/auth"
	"github.com/tobenna/together/server/internal/models"
	"github.com/tobenna/together/server/internal/ws"
)

type transferPresenterRequest struct {
	ToParticipantID string `json:"toParticipantId"`
}

func (s *Server) handleTransferPresenter(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	var req transferPresenterRequest
	if err := readJSON(r, &req); err != nil || req.ToParticipantID == "" {
		writeError(w, http.StatusBadRequest, "Choose who should present.")
		return
	}
	if err := s.Rooms.TransferPresenter(r.Context(), claims.RoomID, claims.Role, claims.ParticipantID, req.ToParticipantID); err != nil {
		mapServiceError(w, err)
		return
	}
	s.syncRoomRoles(r.Context(), claims.RoomID)
	s.Hub.BroadcastToRoom(claims.RoomID, ws.NewEvent(ws.EventPresenterTransfer, claims.RoomID, map[string]string{
		"fromParticipantId": claims.ParticipantID, "toParticipantId": req.ToParticipantID,
	}))
	writeJSON(w, http.StatusOK, map[string]string{"status": "transferred"})
}

type requestActionRequest struct {
	ActionType string `json:"actionType"` 
}

func (s *Server) handleRequestAction(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	var req requestActionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Please try that again.")
		return
	}
	action, err := s.Rooms.RequestAction(r.Context(), claims.RoomID, claims.ParticipantID, models.RoomActionType(req.ActionType))
	if err != nil {
		mapServiceError(w, err)
		return
	}
	s.Hub.BroadcastToRoom(claims.RoomID, ws.NewEvent(ws.EventActionCreated, claims.RoomID, action))
	writeJSON(w, http.StatusCreated, action)
}

func (s *Server) handleListPendingActions(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	list, err := s.Store.ListPendingActions(r.Context(), claims.RoomID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type resolveActionRequest struct {
	Accept bool `json:"accept"`
}

func (s *Server) handleResolveAction(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	var req resolveActionRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Please try that again.")
		return
	}
	actionID := chiActionID(r)
	action, err := s.Rooms.ResolveAction(r.Context(), claims.Role, actionID, req.Accept)
	if err != nil {
		mapServiceError(w, err)
		return
	}

	if req.Accept && (action.ActionType == models.ActionRequestPresenter || action.ActionType == models.ActionRequestStage) {
		if err := s.Rooms.TransferPresenter(r.Context(), claims.RoomID, claims.Role, "", action.ParticipantID); err == nil {
			s.syncRoomRoles(r.Context(), claims.RoomID)
			s.Hub.BroadcastToRoom(claims.RoomID, ws.NewEvent(ws.EventPresenterTransfer, claims.RoomID, map[string]string{
				"toParticipantId": action.ParticipantID,
			}))
		}
	}

	s.Hub.BroadcastToRoom(claims.RoomID, ws.NewEvent(ws.EventActionResolved, claims.RoomID, action))
	writeJSON(w, http.StatusOK, action)
}

func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	var createdBy *string
	if claims.UserID != nil {
		createdBy = claims.UserID
	}
	inv, err := s.Rooms.CreateInvite(r.Context(), claims.RoomID, claims.Role, createdBy)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, inv)
}

func (s *Server) handleLiveKitToken(w http.ResponseWriter, r *http.Request) {
	claims := auth.RoomClaimsFromContext(r.Context())
	room, err := s.Rooms.GetRoom(r.Context(), claims.RoomID)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	if room.Mode != models.RoomModeOnline {
		writeError(w, http.StatusBadRequest, "This is a local room — it doesn't use LiveKit.")
		return
	}
	p, err := s.Store.GetParticipantByID(r.Context(), claims.ParticipantID)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	token, err := s.LiveKit.Mint(room.ID, p.ID, p.DisplayName, p.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't set up screen sharing right now.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token, "url": s.LiveKit.URL()})
}
