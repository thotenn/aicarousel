package db

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// openTestDB opens an isolated SQLite file DB (no migrations applied yet).
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	d, err := Open(path)
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	t.Cleanup(func() { d.Close() }) //nolint:errcheck
	return d
}

func TestUp_createsTables(t *testing.T) {
	d := openTestDB(t)

	if err := Up(d); err != nil {
		t.Fatalf("Up: %v", err)
	}

	for _, table := range []string{"api_keys", "provider_settings", "_migrations"} {
		var n int
		if err := d.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&n); err != nil || n == 0 {
			t.Errorf("table %q missing after Up", table)
		}
	}
}

func TestUp_idempotent(t *testing.T) {
	d := openTestDB(t)

	for i := 0; i < 2; i++ {
		if err := Up(d); err != nil {
			t.Fatalf("Up run %d: %v", i+1, err)
		}
	}
}

func TestUp_defaultProviders(t *testing.T) {
	d := openTestDB(t)
	if err := Up(d); err != nil {
		t.Fatalf("Up: %v", err)
	}

	want := []string{"cerebras", "groq", "openrouter", "gemini"}
	rows, err := d.Query(
		`SELECT provider_key FROM provider_settings ORDER BY priority ASC`,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close() //nolint:errcheck

	var got []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		got = append(got, k)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	if len(got) != len(want) {
		t.Fatalf("default providers: got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("provider[%d]: got %q, want %q", i, got[i], w)
		}
	}
}

func TestDown_rollbacksLast(t *testing.T) {
	d := openTestDB(t)
	if err := Up(d); err != nil {
		t.Fatalf("Up: %v", err)
	}

	// Roll back the last migration (002_create_provider_settings).
	if err := Down(d); err != nil {
		t.Fatalf("Down: %v", err)
	}

	var n int
	d.QueryRow( //nolint:errcheck
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='provider_settings'`,
	).Scan(&n) //nolint:errcheck
	if n != 0 {
		t.Error("provider_settings should be gone after Down")
	}

	// api_keys must still exist.
	d.QueryRow( //nolint:errcheck
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='api_keys'`,
	).Scan(&n) //nolint:errcheck
	if n == 0 {
		t.Error("api_keys should still exist after one Down")
	}
}

func TestDown_noopWhenEmpty(t *testing.T) {
	d := openTestDB(t)
	if err := ensureMigrationsTable(d); err != nil {
		t.Fatal(err)
	}
	if err := Down(d); err != nil {
		t.Errorf("Down on empty table should be noop, got: %v", err)
	}
}

func TestMigrationsRecorded(t *testing.T) {
	d := openTestDB(t)
	if err := Up(d); err != nil {
		t.Fatalf("Up: %v", err)
	}

	var count int
	if err := d.QueryRow(`SELECT COUNT(*) FROM _migrations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 migration records, got %d", count)
	}
}
