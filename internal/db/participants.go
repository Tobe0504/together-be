package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/tobenna/together/server/internal/models"
)

const participantSelect = `
	SELECT id, room_id, user_id, display_name, role, status, is_primary, joined_at, last_seen_at
	FROM room_participants`

func (s *Store) CreateParticipant(ctx context.Context, p *models.RoomParticipant) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO room_participants (id, room_id, user_id, display_name, role, status, is_primary, joined_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.RoomID, p.UserID, p.DisplayName, string(p.Role), string(p.Status), boolToInt(p.IsPrimary),
		p.JoinedAt.Format(time.RFC3339Nano), p.LastSeenAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetParticipantByID(ctx context.Context, id string) (*models.RoomParticipant, error) {
	return s.scanParticipant(s.DB.QueryRowContext(ctx, participantSelect+` WHERE id = ?`, id))
}

func (s *Store) ListParticipants(ctx context.Context, roomID string) ([]models.RoomParticipant, error) {
	rows, err := s.DB.QueryContext(ctx, participantSelect+` WHERE room_id = ? ORDER BY joined_at ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.RoomParticipant
	for rows.Next() {
		var p models.RoomParticipant
		var role, status string
		var isPrimary int
		var joinedAt, lastSeenAt string
		if err := rows.Scan(&p.ID, &p.RoomID, &p.UserID, &p.DisplayName, &role, &status, &isPrimary, &joinedAt, &lastSeenAt); err != nil {
			return nil, err
		}
		p.Role = models.ParticipantRole(role)
		p.Status = models.ParticipantStatus(status)
		p.IsPrimary = isPrimary != 0
		p.JoinedAt, _ = time.Parse(time.RFC3339Nano, joinedAt)
		p.LastSeenAt, _ = time.Parse(time.RFC3339Nano, lastSeenAt)
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpdateParticipantRole(ctx context.Context, id string, role models.ParticipantRole) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE room_participants SET role = ?, last_seen_at = ? WHERE id = ?`,
		string(role), time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *Store) UpdateParticipantStatus(ctx context.Context, id string, status models.ParticipantStatus) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE room_participants SET status = ?, last_seen_at = ? WHERE id = ?`,
		string(status), time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *Store) SetParticipantPrimary(ctx context.Context, roomID, participantID string, primary bool) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE room_participants SET is_primary = ? WHERE id = ? AND room_id = ?`,
		boolToInt(primary), participantID, roomID)
	return err
}

func (s *Store) ClearOtherPrimaries(ctx context.Context, roomID, exceptParticipantID string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE room_participants SET is_primary = 0 WHERE room_id = ? AND id != ?`,
		roomID, exceptParticipantID)
	return err
}

func (s *Store) TouchParticipant(ctx context.Context, id string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE room_participants SET last_seen_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *Store) scanParticipant(row *sql.Row) (*models.RoomParticipant, error) {
	var p models.RoomParticipant
	var role, status string
	var isPrimary int
	var joinedAt, lastSeenAt string
	if err := row.Scan(&p.ID, &p.RoomID, &p.UserID, &p.DisplayName, &role, &status, &isPrimary, &joinedAt, &lastSeenAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	p.Role = models.ParticipantRole(role)
	p.Status = models.ParticipantStatus(status)
	p.IsPrimary = isPrimary != 0
	p.JoinedAt, _ = time.Parse(time.RFC3339Nano, joinedAt)
	p.LastSeenAt, _ = time.Parse(time.RFC3339Nano, lastSeenAt)
	return &p, nil
}
