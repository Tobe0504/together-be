package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/tobenna/together/server/internal/models"
)

func (s *Store) CreateSession(ctx context.Context, sess *models.RoomSession) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO room_sessions (id, room_id, participant_id, token_jti, issued_at, expires_at, revoked)
		VALUES (?, ?, ?, ?, ?, ?, 0)`,
		sess.ID, sess.RoomID, sess.ParticipantID, sess.TokenJTI,
		sess.IssuedAt.Format(time.RFC3339Nano), sess.ExpiresAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) IsSessionValid(ctx context.Context, jti string) (bool, error) {
	var revoked int
	var expiresAt string
	err := s.DB.QueryRowContext(ctx,
		`SELECT revoked, expires_at FROM room_sessions WHERE token_jti = ?`, jti,
	).Scan(&revoked, &expiresAt)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if revoked != 0 {
		return false, nil
	}
	exp, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		return false, err
	}
	return time.Now().UTC().Before(exp), nil
}

func (s *Store) RevokeSessionsForParticipant(ctx context.Context, participantID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE room_sessions SET revoked = 1 WHERE participant_id = ?`, participantID)
	return err
}

func (s *Store) RevokeSessionsForRoom(ctx context.Context, roomID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE room_sessions SET revoked = 1 WHERE room_id = ?`, roomID)
	return err
}
