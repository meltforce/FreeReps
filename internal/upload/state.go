package upload

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// StateDB tracks which files have been successfully uploaded to avoid re-sending.
type StateDB struct {
	db *sql.DB
}

// OpenStateDB opens (or creates) the SQLite state database at dir/state.db.
func OpenStateDB(dir string) (*StateDB, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating state dir %s: %w", dir, err)
	}

	dbPath := filepath.Join(dir, "state.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening state db: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS uploaded_files (
		path        TEXT PRIMARY KEY,
		size        INTEGER NOT NULL,
		hash        TEXT NOT NULL,
		uploaded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("creating state table: %w", err)
	}

	return &StateDB{db: db}, nil
}

// IsUploaded checks if a file has already been uploaded with the same size and hash.
func (s *StateDB) IsUploaded(relPath string, size int64, hash string) (bool, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM uploaded_files WHERE path = ? AND size = ? AND hash = ?`,
		relPath, size, hash,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// MarkUploaded records that a file was successfully uploaded.
func (s *StateDB) MarkUploaded(relPath string, size int64, hash string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO uploaded_files (path, size, hash) VALUES (?, ?, ?)`,
		relPath, size, hash,
	)
	return err
}

// Close closes the state database.
func (s *StateDB) Close() error {
	return s.db.Close()
}

// HashFile computes the SHA-256 hash of a file.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
