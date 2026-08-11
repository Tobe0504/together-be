package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/tobenna/together/server/internal/models"
)

func (s *Store) CreateAction(ctx context.Context, a *models.RoomAction) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO room_actions (id, room_id, participant_id, action_type, status, created_at, resolved_at)
		VALUES (?, ?, ?, ?, ?, ?, NULL)`,
		a.ID, a.RoomID, a.ParticipantID, string(a.ActionType), string(a.Status),
		a.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetActionByID(ctx context.Context, id string) (*models.RoomAction, error) {
	return s.scanAction(s.DB.QueryRowContext(ctx, `
		SELECT id, room_id, participant_id, action_type, status, created_at, resolved_at
		FROM room_actions WHERE id = ?`, id))
}

func (s *Store) ResolveAction(ctx context.Context, id string, status models.RoomActionStatus) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE room_actions SET status = ?, resolved_at = ? WHERE id = ?`,
		string(status), time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *Store) ListPendingActions(ctx context.Context, roomID string) ([]models.RoomAction, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, room_id, participant_id, action_type, status, created_at, resolved_at
		FROM room_actions WHERE room_id = ? AND status = 'PENDING' ORDER BY created_at ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.RoomAction
	for rows.Next() {
		var a models.RoomAction
		var actionType, status, createdAt string
		var resolvedAt sql.NullString
		if err := rows.Scan(&a.ID, &a.RoomID, &a.ParticipantID, &actionType, &status, &createdAt, &resolvedAt); err != nil {
			return nil, err
		}
		a.ActionType = models.RoomActionType(actionType)
		a.Status = models.RoomActionStatus(status)
		a.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		if resolvedAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, resolvedAt.String)
			a.ResolvedAt = &t
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) scanAction(row *sql.Row) (*models.RoomAction, error) {
	var a models.RoomAction
	var actionType, status, createdAt string
	var resolvedAt sql.NullString
	if err := row.Scan(&a.ID, &a.RoomID, &a.ParticipantID, &actionType, &status, &createdAt, &resolvedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	a.ActionType = models.RoomActionType(actionType)
	a.Status = models.RoomActionStatus(status)
	a.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if resolvedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, resolvedAt.String)
		a.ResolvedAt = &t
	}
	return &a, nil
}
