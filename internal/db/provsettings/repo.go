package provsettings

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Setting mirrors the provider_settings table row.
type Setting struct {
	ID          int64
	ProviderKey string
	IsEnabled   int // 0 | 1
	Priority    int
	CreatedAt   string
	UpdatedAt   string
}

// Repo is the data-access interface for provider settings.
type Repo interface {
	All(ctx context.Context) ([]Setting, error)
	Get(ctx context.Context, key string) (*Setting, error)
	Enabled(ctx context.Context) ([]Setting, error)
	EnabledKeys(ctx context.Context) ([]string, error)
	IsEnabled(ctx context.Context, key string) (bool, error)
	Toggle(ctx context.Context, key string, enabled bool) (bool, error)
	UpdatePriority(ctx context.Context, key string, priority int) (bool, error)
	Reorder(ctx context.Context, orderedKeys []string) error
	Ensure(ctx context.Context, key string, priority *int) error
	Sync(ctx context.Context, keys []string) error
}

type repo struct{ db *sql.DB }

// New returns a Repo backed by the given *sql.DB.
func New(db *sql.DB) Repo { return &repo{db: db} }

// All returns every provider setting ordered by priority ASC.
func (r *repo) All(ctx context.Context) ([]Setting, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, provider_key, is_enabled, priority, created_at, updated_at
		   FROM provider_settings ORDER BY priority ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("provsettings: all: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	return scanSettings(rows)
}

// Get returns the setting for a single provider key, or nil if not found.
func (r *repo) Get(ctx context.Context, key string) (*Setting, error) {
	var s Setting
	err := r.db.QueryRowContext(ctx,
		`SELECT id, provider_key, is_enabled, priority, created_at, updated_at
		   FROM provider_settings WHERE provider_key = ?`,
		key,
	).Scan(&s.ID, &s.ProviderKey, &s.IsEnabled, &s.Priority, &s.CreatedAt, &s.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("provsettings: get %q: %w", key, err)
	}
	return &s, nil
}

// Enabled returns all enabled providers ordered by priority ASC.
func (r *repo) Enabled(ctx context.Context) ([]Setting, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, provider_key, is_enabled, priority, created_at, updated_at
		   FROM provider_settings WHERE is_enabled = 1 ORDER BY priority ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("provsettings: enabled: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	return scanSettings(rows)
}

// EnabledKeys returns the provider keys of all enabled providers ordered by priority.
func (r *repo) EnabledKeys(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT provider_key FROM provider_settings WHERE is_enabled = 1 ORDER BY priority ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("provsettings: enabled keys: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("provsettings: enabled keys scan: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// IsEnabled reports whether the provider is enabled.
// Graceful degradation: a missing row OR a missing table is treated as enabled,
// matching the TS behavior where a missing DB row falls back to enabled.
func (r *repo) IsEnabled(ctx context.Context, key string) (bool, error) {
	var enabled int
	err := r.db.QueryRowContext(ctx,
		`SELECT is_enabled FROM provider_settings WHERE provider_key = ?`, key,
	).Scan(&enabled)

	if err == sql.ErrNoRows {
		return true, nil // no row → treat as enabled
	}
	if err != nil {
		if isNoTableErr(err) {
			return true, nil // table missing → treat as enabled
		}
		return false, fmt.Errorf("provsettings: is_enabled %q: %w", key, err)
	}
	return enabled == 1, nil
}

// Toggle sets is_enabled for a provider and updates updated_at.
// Returns false if the provider key is not found.
func (r *repo) Toggle(ctx context.Context, key string, enabled bool) (bool, error) {
	v := 0
	if enabled {
		v = 1
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE provider_settings SET is_enabled = ?, updated_at = datetime('now') WHERE provider_key = ?`,
		v, key,
	)
	if err != nil {
		return false, fmt.Errorf("provsettings: toggle %q: %w", key, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// UpdatePriority changes the priority of a single provider and bumps updated_at.
// Returns false if not found.
func (r *repo) UpdatePriority(ctx context.Context, key string, priority int) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE provider_settings SET priority = ?, updated_at = datetime('now') WHERE provider_key = ?`,
		priority, key,
	)
	if err != nil {
		return false, fmt.Errorf("provsettings: update priority %q: %w", key, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Reorder assigns priority = i+1 to each key in the given ordered list,
// all in a single transaction.
func (r *repo) Reorder(ctx context.Context, orderedKeys []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("provsettings: reorder begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for i, key := range orderedKeys {
		if _, err := tx.ExecContext(ctx,
			`UPDATE provider_settings SET priority = ?, updated_at = datetime('now') WHERE provider_key = ?`,
			i+1, key,
		); err != nil {
			return fmt.Errorf("provsettings: reorder %q: %w", key, err)
		}
	}
	return tx.Commit()
}

// Ensure inserts the provider with INSERT OR IGNORE (idempotent).
// If priority is nil, MAX(priority)+1 is used.
func (r *repo) Ensure(ctx context.Context, key string, priority *int) error {
	p := 1
	if priority != nil {
		p = *priority
	} else {
		var maxP sql.NullInt64
		if err := r.db.QueryRowContext(ctx,
			`SELECT MAX(priority) FROM provider_settings`,
		).Scan(&maxP); err != nil {
			return fmt.Errorf("provsettings: ensure max priority: %w", err)
		}
		if maxP.Valid {
			p = int(maxP.Int64) + 1
		}
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO provider_settings (provider_key, is_enabled, priority) VALUES (?, 1, ?)`,
		key, p,
	)
	if err != nil {
		return fmt.Errorf("provsettings: ensure %q: %w", key, err)
	}
	return nil
}

// Sync reconciles the provider_settings table with the given canonical key list.
// Missing keys are inserted (enabled, MAX+1 priority); stale keys are deleted.
func (r *repo) Sync(ctx context.Context, keys []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("provsettings: sync begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Fetch current state.
	rows, err := tx.QueryContext(ctx, `SELECT provider_key FROM provider_settings`)
	if err != nil {
		return fmt.Errorf("provsettings: sync list: %w", err)
	}
	existing := make(map[string]bool)
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close() //nolint:errcheck
			return fmt.Errorf("provsettings: sync scan: %w", err)
		}
		existing[k] = true
	}
	rows.Close() //nolint:errcheck
	if err := rows.Err(); err != nil {
		return err
	}

	desired := make(map[string]bool, len(keys))
	for _, k := range keys {
		desired[k] = true
	}

	// Add missing providers.
	nextPriority := len(existing) + 1
	for _, k := range keys {
		if existing[k] {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO provider_settings (provider_key, is_enabled, priority) VALUES (?, 1, ?)`,
			k, nextPriority,
		); err != nil {
			return fmt.Errorf("provsettings: sync insert %q: %w", k, err)
		}
		nextPriority++
	}

	// Delete stale providers.
	for k := range existing {
		if desired[k] {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM provider_settings WHERE provider_key = ?`, k,
		); err != nil {
			return fmt.Errorf("provsettings: sync delete %q: %w", k, err)
		}
	}

	return tx.Commit()
}

// scanSettings scans *sql.Rows into a []Setting slice.
func scanSettings(rows *sql.Rows) ([]Setting, error) {
	var out []Setting
	for rows.Next() {
		var s Setting
		if err := rows.Scan(&s.ID, &s.ProviderKey, &s.IsEnabled, &s.Priority,
			&s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("provsettings: scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// isNoTableErr reports whether the error is a "no such table" SQLite error.
func isNoTableErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such table")
}
