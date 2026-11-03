package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrEmailTaken = errors.New("email already registered")
)

type Account struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	TenantID      string `json:"tenant_id"`
	IsOwner       bool   `json:"is_owner"`
	FreeCallLimit int    `json:"free_call_limit"`
	CallsUsed     int    `json:"calls_used"`
	CreatedAt     int64  `json:"created_at"`
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if path == "" {
		path = "data/auth.db"
	}
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	s.DeleteExpiredSessions(context.Background())
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS accounts (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	tenant_id TEXT NOT NULL DEFAULT '',
	is_demo INTEGER NOT NULL DEFAULT 0,
	is_owner INTEGER NOT NULL DEFAULT 0,
	free_call_limit INTEGER NOT NULL DEFAULT 2,
	calls_used INTEGER NOT NULL DEFAULT 0,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT PRIMARY KEY,
	account_id TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_account ON sessions(account_id);
`)
	if err != nil {
		return err
	}
	for _, col := range []struct {
		name string
		ddl  string
	}{
		{"is_owner", "ALTER TABLE accounts ADD COLUMN is_owner INTEGER NOT NULL DEFAULT 0"},
		{"free_call_limit", "ALTER TABLE accounts ADD COLUMN free_call_limit INTEGER NOT NULL DEFAULT 2"},
		{"calls_used", "ALTER TABLE accounts ADD COLUMN calls_used INTEGER NOT NULL DEFAULT 0"},
	} {
		exists, err := s.columnExists(ctx, "accounts", col.name)
		if err != nil {
			return err
		}
		if !exists {
			if _, err := s.db.ExecContext(ctx, col.ddl); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) columnExists(ctx context.Context, table, column string) (bool, error) {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) CountAccounts(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM accounts").Scan(&n)
	return n, err
}

func (s *Store) CreateAccount(ctx context.Context, email, password, tenantID string, isOwner bool) (Account, error) {
	email = normalizeEmail(email)
	if email == "" || password == "" {
		return Account{}, errors.New("email and password required")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Account{}, err
	}
	limit := 2
	if isOwner {
		limit = 0
	}
	a := Account{
		ID:            newID("acct_"),
		Email:         email,
		TenantID:      tenantID,
		IsOwner:       isOwner,
		FreeCallLimit: limit,
		CreatedAt:     time.Now().Unix(),
	}
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO accounts (id, email, password_hash, tenant_id, is_demo, is_owner, free_call_limit, calls_used, created_at) VALUES (?, ?, ?, ?, 0, ?, ?, 0, ?)",
		a.ID, a.Email, hash, a.TenantID, boolInt(a.IsOwner), a.FreeCallLimit, a.CreatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: accounts.email") {
			return Account{}, ErrEmailTaken
		}
		return Account{}, err
	}
	return a, nil
}

func (s *Store) FindByEmail(ctx context.Context, email string) (Account, string, error) {
	var a Account
	var hash string
	var owner int
	err := s.db.QueryRowContext(ctx,
		"SELECT id, email, password_hash, tenant_id, is_owner, free_call_limit, calls_used, created_at FROM accounts WHERE email = ?",
		normalizeEmail(email),
	).Scan(&a.ID, &a.Email, &hash, &a.TenantID, &owner, &a.FreeCallLimit, &a.CallsUsed, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, "", ErrNotFound
	}
	if err != nil {
		return Account{}, "", err
	}
	a.IsOwner = owner == 1
	return a, hash, nil
}

func (s *Store) StartCall(ctx context.Context, accountID string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		"UPDATE accounts SET calls_used = calls_used + 1 WHERE id = ? AND is_owner = 0 AND calls_used < free_call_limit",
		accountID,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	var owner int
	err = s.db.QueryRowContext(ctx, "SELECT is_owner FROM accounts WHERE id = ?", accountID).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	return owner == 1, nil
}

func (s *Store) UpsertOwner(ctx context.Context, email, password, tenantID string) (Account, error) {
	email = normalizeEmail(email)
	existing, _, err := s.FindByEmail(ctx, email)
	if err == nil {
		hash, err := HashPassword(password)
		if err != nil {
			return Account{}, err
		}
		_, err = s.db.ExecContext(ctx,
			"UPDATE accounts SET password_hash = ?, tenant_id = ?, is_owner = 1, free_call_limit = 0, is_demo = 0 WHERE id = ?",
			hash, tenantID, existing.ID,
		)
		if err != nil {
			return Account{}, err
		}
		existing.TenantID = tenantID
		existing.IsOwner = true
		existing.FreeCallLimit = 0
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Account{}, err
	}
	return s.CreateAccount(ctx, email, password, tenantID, true)
}

func (s *Store) PruneNonOwners(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM sessions WHERE account_id IN (SELECT id FROM accounts WHERE is_owner = 0)",
	)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "DELETE FROM accounts WHERE is_owner = 0")
	return err
}

func (s *Store) CreateSession(ctx context.Context, accountID string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO sessions (token_hash, account_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
		hashToken(token), accountID, now.Unix(), now.Add(ttl).Unix(),
	)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) AccountForToken(ctx context.Context, token string) (Account, error) {
	if token == "" {
		return Account{}, ErrNotFound
	}
	var a Account
	var owner int
	err := s.db.QueryRowContext(ctx, `
SELECT a.id, a.email, a.tenant_id, a.is_owner, a.free_call_limit, a.calls_used, a.created_at
FROM sessions s JOIN accounts a ON a.id = s.account_id
WHERE s.token_hash = ? AND s.expires_at > ?`,
		hashToken(token), time.Now().Unix(),
	).Scan(&a.ID, &a.Email, &a.TenantID, &owner, &a.FreeCallLimit, &a.CallsUsed, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, err
	}
	a.IsOwner = owner == 1
	return a, nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = ?", hashToken(token))
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) {
	s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= ?", time.Now().Unix())
}

func (s *Store) SessionCount(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sessions").Scan(&n)
	return n, err
}

func newID(prefix string) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(b)
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
