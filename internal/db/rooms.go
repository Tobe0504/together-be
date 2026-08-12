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
	SELECT id, name, mode, owner_id, status, join_code, access_protected, pin_hash, host_lan_addr, primary_presenter_id, expires_at, created_at, updated_at
	FROM rooms`

func (s *Store) scanRoom(row *sql.Row) (*models.Room, error) {
	var r models.Room
	var mode, status string
	var accessProtected int
	var expiresAt, createdAt, updatedAt string
	if err := row.Scan(&r.ID, &r.Name, &mode, &r.OwnerID, &status, &r.JoinCode, &accessProtected, &r.PinHash, &r.HostLANAddr, &r.PrimaryPresentID, &expiresAt, &createdAt, &updatedAt); err != nil {
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
	if err := rows.Scan(&r.ID, &r.Name, &mode, &r.OwnerID, &status, &r.JoinCode, &accessProtected, &r.PinHash, &r.HostLANAddr, &r.PrimaryPresentID, &expiresAt, &createdAt, &updatedAt); err != nil {
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

// ClaimPresenter marks participantID as the room's live presenter, but only
// if nobody else already holds it. Reports whether the claim succeeded.
//
// The condition lives in the UPDATE's WHERE rather than in a read-then-write
// in Go: two people pressing Present at the same moment would both pass a
// separate "is anyone presenting?" check and then both write, leaving two
// live screens and viewers seeing whichever arrived last. Letting the
// database decide makes exactly one of them win.
func (s *Store) ClaimPresenter(ctx context.Context, roomID, participantID string) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `
		UPDATE rooms SET primary_presenter_id = ?, updated_at = ?
		WHERE id = ? AND (primary_presenter_id IS NULL OR primary_presenter_id = ?)`,
		participantID, time.Now().UTC().Format(time.RFC3339Nano), roomID, participantID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// ReleasePresenter clears the slot, but only if participantID is the one
// holding it — so a straggling "I stopped" from a previous presenter can't
// knock the current one off the stage.
func (s *Store) ReleasePresenter(ctx context.Context, roomID, participantID string) error {
	_, err := s.DB.ExecContext(ctx, `
		UPDATE rooms SET primary_presenter_id = NULL, updated_at = ?
		WHERE id = ? AND primary_presenter_id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), roomID, participantID)
	return err
}

// ReleasePresenterIfHeldBy clears the slot when a participant disconnects or
// is removed mid-share, so the stage doesn't stay locked by someone who has
// left the room.
func (s *Store) ReleasePresenterForParticipant(ctx context.Context, participantID string) error {
	_, err := s.DB.ExecContext(ctx, `
		UPDATE rooms SET primary_presenter_id = NULL, updated_at = ?
		WHERE primary_presenter_id = ?`,
		time.Now().UTC().Format(time.RFC3339Nano), participantID)
	return err
}
