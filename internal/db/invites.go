package db

import (
	"context"
	"time"

	"github.com/tobenna/together/server/internal/models"
)

func (s *Store) CreateInvite(ctx context.Context, inv *models.RoomInvite) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO room_invites (id, room_id, created_by, join_code, expires_at, revoked, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?)`,
		inv.ID, inv.RoomID, inv.CreatedBy, inv.JoinCode,
		inv.ExpiresAt.Format(time.RFC3339Nano), inv.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListInvitesForRoom(ctx context.Context, roomID string) ([]models.RoomInvite, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, room_id, created_by, join_code, expires_at, revoked, created_at
		FROM room_invites WHERE room_id = ? ORDER BY created_at DESC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.RoomInvite
	for rows.Next() {
		var inv models.RoomInvite
		var revoked int
		var expiresAt, createdAt string
		if err := rows.Scan(&inv.ID, &inv.RoomID, &inv.CreatedBy, &inv.JoinCode, &expiresAt, &revoked, &createdAt); err != nil {
			return nil, err
		}
		inv.Revoked = revoked != 0
		inv.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expiresAt)
		inv.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		out = append(out, inv)
	}
	return out, rows.Err()
}
