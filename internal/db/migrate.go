package db

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

//go:embed migrations/CHECKSUMS
var checksumsFile []byte

// init validates the integrity of all embedded SQL migration files against
// the checked-in CHECKSUMS file. The process panics on mismatch — a corrupted
// or missing migration must never silently reach production.
func init() {
	files, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		panic("db: cannot read embedded migrations: " + err.Error())
	}

	expected := parseChecksums(checksumsFile)

	for _, f := range files {
		name := f.Name()
		if name == "CHECKSUMS" {
			continue
		}
		data, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			panic(fmt.Sprintf("db: cannot read embedded migration %q: %v", name, err))
		}
		got := sha256.Sum256(data)
		want, ok := expected[name]
		if !ok {
			panic(fmt.Sprintf("db: migration %q is not listed in CHECKSUMS — run `make checksums`", name))
		}
		if hex.EncodeToString(got[:]) != want {
			panic(fmt.Sprintf("db: migration %q checksum mismatch — run `make checksums`", name))
		}
		delete(expected, name) // consumed; remaining = orphaned entries
	}

	// Entries left in expected have no matching .sql file — CHECKSUMS is stale.
	for name := range expected {
		panic(fmt.Sprintf("db: CHECKSUMS lists %q but no such migration exists", name))
	}
}

// Up runs all pending migrations in order.
func Up(d *sql.DB) error {
	if err := ensureMigrationsTable(d); err != nil {
		return err
	}

	files, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("db: read embedded migrations: %w", err)
	}

	var upFiles []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".up.sql") {
			upFiles = append(upFiles, f.Name())
		}
	}
	sort.Strings(upFiles)

	for _, name := range upFiles {
		applied, err := isMigrationApplied(d, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		data, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("db: read migration %q: %w", name, err)
		}

		tx, err := d.Begin()
		if err != nil {
			return fmt.Errorf("db: begin tx for %q: %w", name, err)
		}

		if _, err := tx.Exec(string(data)); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("db: exec migration %q: %w", name, err)
		}

		if _, err := tx.Exec(
			`INSERT INTO _migrations(name) VALUES (?)`, name,
		); err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("db: record migration %q: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("db: commit migration %q: %w", name, err)
		}
	}
	return nil
}

// Down rolls back the last applied migration.
func Down(d *sql.DB) error {
	var name string
	err := d.QueryRow(
		`SELECT name FROM _migrations ORDER BY id DESC LIMIT 1`,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return nil // nothing to roll back
	}
	if err != nil {
		return fmt.Errorf("db: last migration: %w", err)
	}

	downName := strings.Replace(name, ".up.sql", ".down.sql", 1)
	data, err := migrationsFS.ReadFile("migrations/" + downName)
	if err != nil {
		return fmt.Errorf("db: read down migration %q: %w", downName, err)
	}

	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("db: begin tx for down %q: %w", downName, err)
	}

	if _, err := tx.Exec(string(data)); err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("db: exec down migration %q: %w", downName, err)
	}

	if _, err := tx.Exec(
		`DELETE FROM _migrations WHERE name = ?`, name,
	); err != nil {
		tx.Rollback() //nolint:errcheck
		return fmt.Errorf("db: delete migration record: %w", err)
	}

	return tx.Commit()
}

// ensureMigrationsTable creates the _migrations tracking table if absent.
func ensureMigrationsTable(d *sql.DB) error {
	_, err := d.Exec(`CREATE TABLE IF NOT EXISTS _migrations (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL UNIQUE,
		executed_at TEXT DEFAULT (datetime('now'))
	)`)
	return err
}

// isMigrationApplied reports whether a migration is already recorded.
func isMigrationApplied(d *sql.DB, name string) (bool, error) {
	var count int
	err := d.QueryRow(
		`SELECT COUNT(*) FROM _migrations WHERE name = ?`, name,
	).Scan(&count)
	return count > 0, err
}

// parseChecksums parses a CHECKSUMS file into a map[filename]sha256hex.
// Format: one entry per line — "<sha256hex>  <filename>" (two spaces).
func parseChecksums(data []byte) map[string]string {
	m := make(map[string]string)
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			panic("db: malformed CHECKSUMS line: " + line)
		}
		filename := strings.TrimSpace(parts[1])
		checksum := strings.TrimSpace(parts[0])
		m[filename] = checksum
	}
	return m
}
