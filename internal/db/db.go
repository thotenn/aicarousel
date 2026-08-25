package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) the SQLite database at path.
// Required pragmas are applied via the DSN:
//   - journal_mode=WAL : concurrent readers + single serialised writer.
//   - foreign_keys=1   : enforce referential integrity.
//   - busy_timeout=5000: wait up to 5 s on a locked DB instead of returning
//     SQLITE_BUSY immediately (needed because every authenticated request
//     performs a write to bump last_used_at / usage_count).
//
// The parent directory is created if it does not exist.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("db: mkdir %s: %w", filepath.Dir(path), err)
	}
	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)",
		path,
	)
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	if err := d.Ping(); err != nil {
		d.Close() //nolint:errcheck
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return d, nil
}
