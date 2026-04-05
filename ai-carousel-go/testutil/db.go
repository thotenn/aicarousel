package testutil

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/thotenn/aicarousel-go/internal/db"
)

// NewTestDB opens a fresh SQLite database in a temporary directory with all
// migrations applied. Each call returns an isolated database; the file is
// deleted when the test (or subtest) ends.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")

	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("testutil.NewTestDB: open: %v", err)
	}

	if err := db.Up(d); err != nil {
		d.Close() //nolint:errcheck
		t.Fatalf("testutil.NewTestDB: migrate up: %v", err)
	}

	t.Cleanup(func() { d.Close() }) //nolint:errcheck
	return d
}
