package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/tobenna/together/server/internal/models"
)

func (s *Store) CreateRoom(ctx context.Context, r *models.Room) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO rooms (id, name, mode, owner_id, status, join_code, access_protected, pin_hash, host_lan_addr, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, string(r.Mode), r.OwnerID, string(r.Status), r.JoinCode, boolToInt(r.AccessProtected), r.PinHash, r.HostLANAddr,
		r.ExpiresAt.Format(time.RFC3339Nano), r.CreatedAt.Format(time.RFC3339Nano), r.UpdatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetRoomByID(ctx context.Context, id string) (*models.Room, error) {
	return s.scanRoom(s.DB.QueryRowContext(ctx, roomSelect+` WHERE id = ?`, id))
}

func (s *Store) GetRoomByJoinCode(ctx context.Context, code string) (*models.Room, error) {
	return s.scanRoom(s.DB.QueryRowContext(ctx, roomSelect+` WHERE join_code = ?`, code))
}

func (s *Store) UpdateRoomStatus(ctx context.Context, id string, status models.RoomStatus) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE rooms SET status = ?, updated_at = ? WHERE id = ?`,
		string(status), time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *Store) UpdateRoomAccess(ctx context.Context, id string, name string, accessProtected bool, pinHash *string) error {
	_, err := s.DB.ExecContext(ctx, `
		UPDATE rooms SET name = ?, access_protected = ?, pin_hash = ?, updated_at = ? WHERE id = ?`,
		name, boolToInt(accessProtected), pinHash, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *Store) SetRoomHostLANAddr(ctx context.Context, id, addr string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE rooms SET host_lan_addr = ?, updated_at = ? WHERE id = ?`,
		addr, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *Store) RoomHistoryForUser(ctx context.Context, userID string, limit int) ([]models.Room, error) {
	rows, err := s.DB.QueryContext(ctx, roomSelect+`
		WHERE id IN (
			SELECT DISTINCT room_id FROM room_participants WHERE user_id = ?
		)
		ORDER BY created_at DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Room
	for rows.Next() {
		r, err := s.scanRoomRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

const roomSelect = `
	SELECT id, name, mode, owner_id, status, join_code, access_protected, pin_hash, host_lan_addr, expires_at, created_at, updated_at
	FROM rooms`

func (s *Store) scanRoom(row *sql.Row) (*models.Room, error) {
	var r models.Room
	var mode, status string
	var accessProtected int
	var expiresAt, createdAt, updatedAt string
	if err := row.Scan(&r.ID, &r.Name, &mode, &r.OwnerID, &status, &r.JoinCode, &accessProtected, &r.PinHash, &r.HostLANAddr, &expiresAt, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	r.Mode = models.RoomMode(mode)
	r.Status = models.RoomStatus(status)
	r.AccessProtected = accessProtected != 0
	r.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expiresAt)
	r.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	r.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return &r, nil
}

func (s *Store) scanRoomRows(rows *sql.Rows) (*models.Room, error) {
	var r models.Room
	var mode, status string
	var accessProtected int
	var expiresAt, createdAt, updatedAt string
	if err := rows.Scan(&r.ID, &r.Name, &mode, &r.OwnerID, &status, &r.JoinCode, &accessProtected, &r.PinHash, &r.HostLANAddr, &expiresAt, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	r.Mode = models.RoomMode(mode)
	r.Status = models.RoomStatus(status)
	r.AccessProtected = accessProtected != 0
	r.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expiresAt)
	r.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	r.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return &r, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
