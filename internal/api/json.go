package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/tobenna/together/server/internal/db"
	"github.com/tobenna/together/server/internal/rooms"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}


func writeInternalError(w http.ResponseWriter, err error) {
	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "Something went wrong on our end.")
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func mapServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rooms.ErrForbidden):
		writeError(w, http.StatusForbidden, "You don't have permission to do that.")
	case errors.Is(err, rooms.ErrNotFound), errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "We couldn't find that room.")
	case errors.Is(err, rooms.ErrRoomEnded):
		writeError(w, http.StatusGone, "This room has ended.")
	case errors.Is(err, rooms.ErrRoomExpired):
		writeError(w, http.StatusGone, "This room has expired.")
	case errors.Is(err, rooms.ErrRoomFull):
		writeError(w, http.StatusConflict,
			"This room is full. Local rooms are limited to 12 people because each person's screen is sent directly to every other device.")
	case errors.Is(err, rooms.ErrBadAccessCode):
		writeError(w, http.StatusUnauthorized, "That access code isn't right.")
	default:
		writeInternalError(w, err)
	}
}
