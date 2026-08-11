package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/tobenna/together/server/internal/auth"
	"github.com/tobenna/together/server/internal/models"
)


type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Please check your details and try again.")
		return
	}
	if req.Email == "" || req.Password == "" || req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "Email, password, and name are all required.")
		return
	}
	if _, err := s.Store.GetUserByEmail(r.Context(), req.Email); err == nil {
		writeError(w, http.StatusConflict, "An account with that email already exists.")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	now := time.Now().UTC()
	user := &models.User{
		ID: uuid.NewString(), Email: &req.Email, PasswordHash: &hash,
		DisplayName: req.DisplayName, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Store.CreateUser(r.Context(), user); err != nil {
		writeInternalError(w, err)
		return
	}

	token, err := s.Signer.MintAccountToken(user.ID, now.Add(30*24*time.Hour))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"user": user, "token": token})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Please check your details and try again.")
		return
	}
	user, err := s.Store.GetUserByEmail(r.Context(), req.Email)
	if err != nil || user.PasswordHash == nil || !auth.VerifyPassword(*user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "That email or password isn't right.")
		return
	}
	token, err := s.Signer.MintAccountToken(user.ID, time.Now().UTC().Add(30*24*time.Hour))
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "token": token})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	claims := auth.AccountClaimsFromContext(r.Context())
	user, err := s.Store.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "We couldn't find your account.")
		return
	}
	history, err := s.Store.RoomHistoryForUser(r.Context(), user.ID, 20)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "roomHistory": history})
}

func chiRoomID(r *http.Request) string        { return chi.URLParam(r, "roomId") }
func chiParticipantID(r *http.Request) string { return chi.URLParam(r, "participantId") }
func chiActionID(r *http.Request) string      { return chi.URLParam(r, "actionId") }
func chiJoinCode(r *http.Request) string      { return chi.URLParam(r, "code") }
