package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/tobenna/together/server/internal/models"
)

func (s *Store) CreateUser(ctx context.Context, u *models.User) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, display_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		u.ID, u.Email, u.PasswordHash, u.DisplayName,
		u.CreatedAt.Format(time.RFC3339Nano), u.UpdatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	return s.scanUser(s.DB.QueryRowContext(ctx, `
		SELECT id, email, password_hash, display_name, created_at, updated_at
		FROM users WHERE id = ?`, id))
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.scanUser(s.DB.QueryRowContext(ctx, `
		SELECT id, email, password_hash, display_name, created_at, updated_at
		FROM users WHERE email = ?`, email))
}

func (s *Store) scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	var createdAt, updatedAt string
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.DisplayName, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	u.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return &u, nil
}
