package db

import (
	"database/sql"
	"errors"

	"modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")
func IsUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}

	switch sqliteErr.Code() {
	case 2067, 
		1555: 
		return true
	default:
		return false
	}
}

type Store struct {
	DB *sql.DB
}

func NewStore(sqlDB *sql.DB) *Store {
	return &Store{DB: sqlDB}
}
