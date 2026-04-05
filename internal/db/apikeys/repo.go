package apikeys

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/thotenn/aicarousel/internal/util/secure"
)

// ApiKey mirrors the api_keys table row.
type ApiKey struct {
	ID         int64
	KeyHash    string
	KeyPrefix  string
	Name       sql.NullString
	CreatedAt  string
	LastUsedAt sql.NullString
	IsActive   int   // 0 | 1
	UsageCount int64
}

// Repo is the data-access interface for API keys.
type Repo interface {
	Create(ctx context.Context, name string) (plainKey string, record ApiKey, err error)
	Validate(ctx context.Context, plainKey string) (*ApiKey, error) // nil,nil = invalid/not found
	List(ctx context.Context) ([]ApiKey, error)                    // key_hash excluded
	Revoke(ctx context.Context, id int64) (bool, error)
	Delete(ctx context.Context, id int64) (bool, error)
	GetByID(ctx context.Context, id int64) (*ApiKey, error)
}

type repo struct{ db *sql.DB }

// New returns a Repo backed by the given *sql.DB.
func New(db *sql.DB) Repo { return &repo{db: db} }

// Create generates a new API key, persists its hash and prefix, and returns
// the plain-text key (shown to the caller exactly once).
func (r *repo) Create(ctx context.Context, name string) (string, ApiKey, error) {
	plain, hash, prefix := secure.GenerateAPIKey()

	var n sql.NullString
	if name != "" {
		n = sql.NullString{String: name, Valid: true}
	}

	res, err := r.db.ExecContext(ctx,
		`INSERT INTO api_keys (key_hash, key_prefix, name) VALUES (?, ?, ?)`,
		hash, prefix, n,
	)
	if err != nil {
		return "", ApiKey{}, fmt.Errorf("apikeys: create: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return "", ApiKey{}, fmt.Errorf("apikeys: last insert id: %w", err)
	}

	rec, err := r.GetByID(ctx, id)
	if err != nil {
		return "", ApiKey{}, err
	}
	return plain, *rec, nil
}

// Validate looks up a plain-text API key. Returns nil, nil when the key is
// not found or inactive. On success it bumps last_used_at and usage_count.
func (r *repo) Validate(ctx context.Context, plainKey string) (*ApiKey, error) {
	if !strings.HasPrefix(plainKey, "sk-") {
		return nil, nil
	}

	hash := secure.HashKey(plainKey)

	var k ApiKey
	err := r.db.QueryRowContext(ctx,
		`SELECT id, key_hash, key_prefix, name, created_at, last_used_at, is_active, usage_count
		   FROM api_keys WHERE key_hash = ? AND is_active = 1`,
		hash,
	).Scan(&k.ID, &k.KeyHash, &k.KeyPrefix, &k.Name,
		&k.CreatedAt, &k.LastUsedAt, &k.IsActive, &k.UsageCount)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("apikeys: validate lookup: %w", err)
	}

	if _, err := r.db.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = datetime('now'), usage_count = usage_count + 1 WHERE id = ?`,
		k.ID,
	); err != nil {
		return nil, fmt.Errorf("apikeys: validate bump: %w", err)
	}

	return &k, nil
}

// List returns all API key records without the key_hash field.
func (r *repo) List(ctx context.Context) ([]ApiKey, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, key_prefix, name, created_at, last_used_at, is_active, usage_count
		   FROM api_keys ORDER BY id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("apikeys: list: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var keys []ApiKey
	for rows.Next() {
		var k ApiKey
		if err := rows.Scan(&k.ID, &k.KeyPrefix, &k.Name,
			&k.CreatedAt, &k.LastUsedAt, &k.IsActive, &k.UsageCount); err != nil {
			return nil, fmt.Errorf("apikeys: list scan: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// Revoke sets is_active=0 for the given key. Returns false if not found.
func (r *repo) Revoke(ctx context.Context, id int64) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE api_keys SET is_active = 0 WHERE id = ?`, id,
	)
	if err != nil {
		return false, fmt.Errorf("apikeys: revoke: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Delete removes the key row. Returns false if not found.
func (r *repo) Delete(ctx context.Context, id int64) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM api_keys WHERE id = ?`, id,
	)
	if err != nil {
		return false, fmt.Errorf("apikeys: delete: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// GetByID returns a single key record.
func (r *repo) GetByID(ctx context.Context, id int64) (*ApiKey, error) {
	var k ApiKey
	err := r.db.QueryRowContext(ctx,
		`SELECT id, key_hash, key_prefix, name, created_at, last_used_at, is_active, usage_count
		   FROM api_keys WHERE id = ?`,
		id,
	).Scan(&k.ID, &k.KeyHash, &k.KeyPrefix, &k.Name,
		&k.CreatedAt, &k.LastUsedAt, &k.IsActive, &k.UsageCount)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("apikeys: get by id: %w", err)
	}
	return &k, nil
}
