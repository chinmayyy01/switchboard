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
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrEmailTaken       = errors.New("email already registered")
	ErrInvalidToken     = errors.New("invalid or expired token")
	ErrInvalidRole      = errors.New("invalid role")
	ErrEmailNotVerified = errors.New("email not verified")
)

const (
	RoleOwner   = "owner"
	RoleManager = "manager"
	RoleViewer  = "viewer"
)

type Account struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name,omitempty"`
	TenantID      string `json:"tenant_id"`
	Role          string `json:"role"`
	IsOwner       bool   `json:"is_owner"`
	FreeCallLimit int    `json:"free_call_limit"`
	CallsUsed     int    `json:"calls_used"`
	CreatedAt     int64  `json:"created_at"`
	EmailVerified bool   `json:"email_verified"`
}

type Invite struct {
	Email     string `json:"email"`
	TenantID  string `json:"tenant_id"`
	Role      string `json:"role"`
	ExpiresAt int64  `json:"expires_at"`
}

type PendingInvite struct {
	Email     string `json:"email"`
	TenantID  string `json:"tenant_id"`
	Role      string `json:"role"`
	ExpiresAt int64  `json:"expires_at"`
	CreatedAt int64  `json:"created_at"`
	Accepted  bool   `json:"accepted"`
}

func ValidRole(role string) bool {
	switch role {
	case RoleOwner, RoleManager, RoleViewer:
		return true
	default:
		return false
	}
}

func roleFor(isOwner bool, role string) string {
	if isOwner {
		return RoleOwner
	}
	if ValidRole(role) {
		return role
	}
	return RoleManager
}

type Store struct {
	db       *sql.DB
	postgres bool
}

type ExportAccount struct {
	Account
	PasswordHash string
}

// ExportAccounts is used by cmd/migrate-auth to move SQLite accounts to
// Postgres. Sessions are intentionally not exported; users sign in again.
func (s *Store) ExportAccounts(ctx context.Context) ([]ExportAccount, error) {
	rows, err := s.queryRows(ctx, `
SELECT id, email, password_hash, tenant_id, is_owner, free_call_limit, calls_used, created_at,
       name, role, email_verified_at
FROM accounts ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExportAccount
	for rows.Next() {
		var e ExportAccount
		var owner int
		var verified sql.NullInt64
		if err := rows.Scan(&e.ID, &e.Email, &e.PasswordHash, &e.TenantID, &owner,
			&e.FreeCallLimit, &e.CallsUsed, &e.CreatedAt, &e.Name, &e.Role, &verified); err != nil {
			return nil, err
		}
		e.IsOwner = owner == 1
		e.Role = roleFor(e.IsOwner, e.Role)
		e.EmailVerified = verified.Valid
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) ImportAccount(ctx context.Context, e ExportAccount) error {
	var verifiedAt any
	if e.EmailVerified {
		verifiedAt = e.CreatedAt
	}
	_, err := s.exec(ctx, `
INSERT INTO accounts (id, email, password_hash, tenant_id, is_demo, is_owner, free_call_limit, calls_used, created_at, name, role, email_verified_at)
VALUES (?, ?, ?, ?, 0, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (email) DO UPDATE SET
    password_hash = EXCLUDED.password_hash,
    tenant_id = EXCLUDED.tenant_id,
    is_owner = EXCLUDED.is_owner,
    role = EXCLUDED.role,
    free_call_limit = EXCLUDED.free_call_limit,
    calls_used = EXCLUDED.calls_used`,
		e.ID, e.Email, e.PasswordHash, e.TenantID, boolInt(e.IsOwner),
		e.FreeCallLimit, e.CallsUsed, e.CreatedAt, e.Name, roleFor(e.IsOwner, e.Role), verifiedAt)
	return err
}

// OpenDefault uses Postgres when DATABASE_URL is configured, otherwise SQLite.
// AUTH_DRIVER=sqlite forces the local file for development.
func OpenDefault() (*Store, error) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("AUTH_DRIVER")), "sqlite") {
		return Open(os.Getenv("AUTH_DB"))
	}
	if dsn := strings.TrimSpace(os.Getenv("DATABASE_URL")); dsn != "" {
		return OpenPostgres(dsn)
	}
	return Open(os.Getenv("AUTH_DB"))
}

func OpenPostgres(dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("DATABASE_URL is required for Postgres auth")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{db: db, postgres: true}
	if err := s.migratePostgres(ctx); err != nil {
		db.Close()
		return nil, err
	}
	s.DeleteExpiredSessions(ctx)
	return s, nil
}

func (s *Store) query(q string) string {
	if !s.postgres {
		return q
	}
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			b.WriteString("$")
			b.WriteString(strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (s *Store) exec(ctx context.Context, q string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, s.query(q), args...)
}

func (s *Store) queryRow(ctx context.Context, q string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, s.query(q), args...)
}

func (s *Store) queryRows(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, s.query(q), args...)
}

func (s *Store) migratePostgres(ctx context.Context) error {
	_, err := s.exec(ctx, `
CREATE TABLE IF NOT EXISTS accounts (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	tenant_id TEXT NOT NULL DEFAULT '',
	is_demo INTEGER NOT NULL DEFAULT 0,
	is_owner INTEGER NOT NULL DEFAULT 0,
	free_call_limit INTEGER NOT NULL DEFAULT 2,
	calls_used INTEGER NOT NULL DEFAULT 0,
	created_at BIGINT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	token_hash TEXT PRIMARY KEY,
	account_id TEXT NOT NULL,
	created_at BIGINT NOT NULL,
	expires_at BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_account ON sessions(account_id);
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS is_owner INTEGER NOT NULL DEFAULT 0;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS free_call_limit INTEGER NOT NULL DEFAULT 2;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS calls_used INTEGER NOT NULL DEFAULT 0;
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS name TEXT NOT NULL DEFAULT '';
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT '';
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS email_verified_at BIGINT;
CREATE TABLE IF NOT EXISTS invites (
	token_hash TEXT PRIMARY KEY,
	email TEXT NOT NULL,
	tenant_id TEXT NOT NULL,
	role TEXT NOT NULL,
	invited_by TEXT NOT NULL DEFAULT '',
	expires_at BIGINT NOT NULL,
	accepted_at BIGINT,
	created_at BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_invites_email ON invites(email);
CREATE TABLE IF NOT EXISTS auth_tokens (
	token_hash TEXT PRIMARY KEY,
	account_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	expires_at BIGINT NOT NULL,
	used_at BIGINT,
	created_at BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_auth_tokens_account ON auth_tokens(account_id, kind);
UPDATE accounts SET role = 'owner' WHERE is_owner = 1 AND role = '';
UPDATE accounts SET role = 'manager' WHERE is_owner = 0 AND role = '';
UPDATE accounts SET email_verified_at = created_at WHERE email_verified_at IS NULL;
`)
	return err
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
	_, err := s.exec(ctx, `
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
CREATE TABLE IF NOT EXISTS invites (
	token_hash TEXT PRIMARY KEY,
	email TEXT NOT NULL,
	tenant_id TEXT NOT NULL,
	role TEXT NOT NULL,
	invited_by TEXT NOT NULL DEFAULT '',
	expires_at INTEGER NOT NULL,
	accepted_at INTEGER,
	created_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS auth_tokens (
	token_hash TEXT PRIMARY KEY,
	account_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	used_at INTEGER,
	created_at INTEGER NOT NULL
);
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
		{"name", "ALTER TABLE accounts ADD COLUMN name TEXT NOT NULL DEFAULT ''"},
		{"role", "ALTER TABLE accounts ADD COLUMN role TEXT NOT NULL DEFAULT ''"},
		{"email_verified_at", "ALTER TABLE accounts ADD COLUMN email_verified_at INTEGER"},
	} {
		exists, err := s.columnExists(ctx, "accounts", col.name)
		if err != nil {
			return err
		}
		if !exists {
			if _, err := s.exec(ctx, col.ddl); err != nil {
				return err
			}
		}
	}
	_, _ = s.exec(ctx, "UPDATE accounts SET role = 'owner' WHERE is_owner = 1 AND role = ''")
	_, _ = s.exec(ctx, "UPDATE accounts SET role = 'manager' WHERE is_owner = 0 AND role = ''")
	_, _ = s.exec(ctx, "UPDATE accounts SET email_verified_at = created_at WHERE email_verified_at IS NULL")
	return nil
}

func (s *Store) columnExists(ctx context.Context, table, column string) (bool, error) {
	rows, err := s.queryRows(ctx, "PRAGMA table_info("+table+")")
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
	err := s.queryRow(ctx, "SELECT COUNT(*) FROM accounts").Scan(&n)
	return n, err
}

func (s *Store) CreateAccount(ctx context.Context, email, password, tenantID string, isOwner bool) (Account, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return Account{}, err
	}
	role := RoleViewer
	if isOwner {
		role = RoleOwner
	}
	return s.createAccountHash(ctx, "", email, hash, tenantID, role, true, isOwner)
}

func (s *Store) CreateAccountWithRole(ctx context.Context, name, email, password, tenantID, role string, verified bool) (Account, error) {
	if !ValidRole(role) || role == RoleOwner {
		return Account{}, ErrInvalidRole
	}
	hash, err := HashPassword(password)
	if err != nil {
		return Account{}, err
	}
	return s.createAccountHash(ctx, name, email, hash, tenantID, role, verified, false)
}

func (s *Store) CreateOwnerAccount(ctx context.Context, name, email, password, tenantID string, verified bool) (Account, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return Account{}, err
	}
	return s.createAccountHash(ctx, name, email, hash, tenantID, RoleOwner, verified, false)
}

func (s *Store) createAccountHash(ctx context.Context, name, email, hash, tenantID, role string, verified, platformOwner bool) (Account, error) {
	email = normalizeEmail(email)
	if email == "" || hash == "" {
		return Account{}, errors.New("email and password required")
	}
	role = roleFor(platformOwner, role)
	limit := 2
	if platformOwner || role == RoleOwner || role == RoleManager {
		limit = 0
	}
	a := Account{
		ID:            newID("acct_"),
		Email:         email,
		Name:          strings.TrimSpace(name),
		TenantID:      tenantID,
		Role:          role,
		IsOwner:       platformOwner,
		FreeCallLimit: limit,
		CreatedAt:     time.Now().Unix(),
		EmailVerified: verified,
	}
	var verifiedAt any
	if verified {
		verifiedAt = a.CreatedAt
	}
	_, err := s.exec(ctx,
		"INSERT INTO accounts (id, email, password_hash, tenant_id, is_demo, is_owner, free_call_limit, calls_used, created_at, name, role, email_verified_at) VALUES (?, ?, ?, ?, 0, ?, ?, 0, ?, ?, ?, ?)",
		a.ID, a.Email, hash, a.TenantID, boolInt(a.IsOwner), a.FreeCallLimit, a.CreatedAt, a.Name, a.Role, verifiedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: accounts.email") ||
			strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
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
	var verified sql.NullInt64
	err := s.queryRow(ctx,
		"SELECT id, email, password_hash, tenant_id, is_owner, free_call_limit, calls_used, created_at, name, role, email_verified_at FROM accounts WHERE email = ?",
		normalizeEmail(email),
	).Scan(&a.ID, &a.Email, &hash, &a.TenantID, &owner, &a.FreeCallLimit, &a.CallsUsed, &a.CreatedAt,
		&a.Name, &a.Role, &verified)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, "", ErrNotFound
	}
	if err != nil {
		return Account{}, "", err
	}
	a.IsOwner = owner == 1
	a.Role = roleFor(a.IsOwner, a.Role)
	a.EmailVerified = verified.Valid
	return a, hash, nil
}

func (s *Store) StartCall(ctx context.Context, accountID string) (bool, error) {
	res, err := s.exec(ctx,
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
	var owner, limit int
	err = s.queryRow(ctx, "SELECT is_owner, free_call_limit FROM accounts WHERE id = ?", accountID).Scan(&owner, &limit)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	return owner == 1 || limit == 0, nil
}

func (s *Store) UpsertOwner(ctx context.Context, email, password, tenantID string) (Account, error) {
	email = normalizeEmail(email)
	existing, _, err := s.FindByEmail(ctx, email)
	if err == nil {
		hash, err := HashPassword(password)
		if err != nil {
			return Account{}, err
		}
		_, err = s.exec(ctx,
			"UPDATE accounts SET password_hash = ?, tenant_id = ?, is_owner = 1, role = 'owner', email_verified_at = COALESCE(email_verified_at, created_at), free_call_limit = 0, is_demo = 0 WHERE id = ?",
			hash, tenantID, existing.ID,
		)
		if err != nil {
			return Account{}, err
		}
		existing.TenantID = tenantID
		existing.IsOwner = true
		existing.Role = RoleOwner
		existing.FreeCallLimit = 0
		existing.EmailVerified = true
		return existing, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return Account{}, err
	}
	return s.CreateAccount(ctx, email, password, tenantID, true)
}

func (s *Store) PruneNonOwners(ctx context.Context) error {
	_, err := s.exec(ctx,
		"DELETE FROM sessions WHERE account_id IN (SELECT id FROM accounts WHERE is_owner = 0)",
	)
	if err != nil {
		return err
	}
	_, err = s.exec(ctx, "DELETE FROM accounts WHERE is_owner = 0")
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
	_, err = s.exec(ctx,
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
	var verified sql.NullInt64
	err := s.queryRow(ctx, `
SELECT a.id, a.email, a.name, a.tenant_id, a.is_owner, a.role, a.free_call_limit, a.calls_used, a.created_at, a.email_verified_at
FROM sessions s JOIN accounts a ON a.id = s.account_id
WHERE s.token_hash = ? AND s.expires_at > ?`,
		hashToken(token), time.Now().Unix(),
	).Scan(&a.ID, &a.Email, &a.Name, &a.TenantID, &owner, &a.Role, &a.FreeCallLimit, &a.CallsUsed, &a.CreatedAt, &verified)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, err
	}
	a.IsOwner = owner == 1
	a.Role = roleFor(a.IsOwner, a.Role)
	a.EmailVerified = verified.Valid
	return a, nil
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.exec(ctx, "DELETE FROM sessions WHERE token_hash = ?", hashToken(token))
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context) {
	s.exec(ctx, "DELETE FROM sessions WHERE expires_at <= ?", time.Now().Unix())
}

func (s *Store) SessionCount(ctx context.Context) (int, error) {
	var n int
	err := s.queryRow(ctx, "SELECT COUNT(*) FROM sessions").Scan(&n)
	return n, err
}

func (s *Store) CreateInvite(ctx context.Context, email, tenantID, role, invitedBy string, ttl time.Duration) (string, error) {
	email = normalizeEmail(email)
	if email == "" || tenantID == "" || !ValidRole(role) || role == RoleOwner {
		return "", ErrInvalidRole
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	_, err = s.exec(ctx, `
INSERT INTO invites (token_hash, email, tenant_id, role, invited_by, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		hashToken(token), email, tenantID, role, invitedBy, now+int64(ttl.Seconds()), now)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) InviteByToken(ctx context.Context, token string) (Invite, error) {
	if token == "" {
		return Invite{}, ErrInvalidToken
	}
	var invite Invite
	var accepted sql.NullInt64
	err := s.queryRow(ctx, `
SELECT email, tenant_id, role, expires_at, accepted_at
FROM invites WHERE token_hash = ?`, hashToken(token),
	).Scan(&invite.Email, &invite.TenantID, &invite.Role, &invite.ExpiresAt, &accepted)
	if errors.Is(err, sql.ErrNoRows) {
		return Invite{}, ErrInvalidToken
	}
	if err != nil {
		return Invite{}, err
	}
	if accepted.Valid || invite.ExpiresAt <= time.Now().Unix() {
		return Invite{}, ErrInvalidToken
	}
	return invite, nil
}

func (s *Store) ListInvites(ctx context.Context, tenantID string) ([]PendingInvite, error) {
	rows, err := s.queryRows(ctx, `
SELECT email, tenant_id, role, expires_at, created_at, accepted_at
FROM invites WHERE (? = '' OR tenant_id = ?)
ORDER BY created_at DESC LIMIT 100`, tenantID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PendingInvite
	for rows.Next() {
		var invite PendingInvite
		var accepted sql.NullInt64
		if err := rows.Scan(&invite.Email, &invite.TenantID, &invite.Role,
			&invite.ExpiresAt, &invite.CreatedAt, &accepted); err != nil {
			return nil, err
		}
		invite.Accepted = accepted.Valid
		out = append(out, invite)
	}
	return out, rows.Err()
}

func (s *Store) AcceptInvite(ctx context.Context, token string) error {
	result, err := s.exec(ctx, `
UPDATE invites SET accepted_at = ?
WHERE token_hash = ? AND accepted_at IS NULL AND expires_at > ?`,
		time.Now().Unix(), hashToken(token), time.Now().Unix())
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrInvalidToken
	}
	return nil
}

func (s *Store) CreateAuthToken(ctx context.Context, accountID, kind string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = time.Hour
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	now := time.Now().Unix()
	_, _ = s.exec(ctx,
		"UPDATE auth_tokens SET used_at = ? WHERE account_id = ? AND kind = ? AND used_at IS NULL",
		now, accountID, kind)
	_, err = s.exec(ctx, `
INSERT INTO auth_tokens (token_hash, account_id, kind, expires_at, created_at)
VALUES (?, ?, ?, ?, ?)`,
		hashToken(token), accountID, kind, now+int64(ttl.Seconds()), now)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Store) ConsumeAuthToken(ctx context.Context, token, kind string) (string, error) {
	if token == "" {
		return "", ErrInvalidToken
	}
	var accountID string
	var expiresAt int64
	var used sql.NullInt64
	err := s.queryRow(ctx, `
SELECT account_id, expires_at, used_at FROM auth_tokens
WHERE token_hash = ? AND kind = ?`, hashToken(token), kind,
	).Scan(&accountID, &expiresAt, &used)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrInvalidToken
	}
	if err != nil {
		return "", err
	}
	if used.Valid || expiresAt <= time.Now().Unix() {
		return "", ErrInvalidToken
	}
	result, err := s.exec(ctx,
		"UPDATE auth_tokens SET used_at = ? WHERE token_hash = ? AND used_at IS NULL",
		time.Now().Unix(), hashToken(token))
	if err != nil {
		return "", err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", ErrInvalidToken
	}
	return accountID, nil
}

func (s *Store) MarkEmailVerified(ctx context.Context, accountID string) error {
	_, err := s.exec(ctx,
		"UPDATE accounts SET email_verified_at = COALESCE(email_verified_at, ?) WHERE id = ?",
		time.Now().Unix(), accountID)
	return err
}

func (s *Store) SetPassword(ctx context.Context, accountID, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.exec(ctx, "UPDATE accounts SET password_hash = ? WHERE id = ?", hash, accountID)
	return err
}

func (s *Store) PasswordHash(ctx context.Context, accountID string) (string, error) {
	var hash string
	err := s.queryRow(ctx, "SELECT password_hash FROM accounts WHERE id = ?", accountID).Scan(&hash)
	return hash, err
}

func (s *Store) DeleteSessionsForAccount(ctx context.Context, accountID string) error {
	_, err := s.exec(ctx, "DELETE FROM sessions WHERE account_id = ?", accountID)
	return err
}

// DeleteOtherSessions signs out every session except keepToken. An empty
// keepToken signs out all sessions.
func (s *Store) DeleteOtherSessions(ctx context.Context, accountID, keepToken string) error {
	if keepToken == "" {
		return s.DeleteSessionsForAccount(ctx, accountID)
	}
	_, err := s.exec(ctx,
		"DELETE FROM sessions WHERE account_id = ? AND token_hash <> ?",
		accountID, hashToken(keepToken))
	return err
}

func (s *Store) AccountByID(ctx context.Context, id string) (Account, error) {
	var a Account
	var owner int
	var verified sql.NullInt64
	err := s.queryRow(ctx, `
SELECT id, email, name, tenant_id, is_owner, role, free_call_limit, calls_used, created_at, email_verified_at
FROM accounts WHERE id = ?`, id,
	).Scan(&a.ID, &a.Email, &a.Name, &a.TenantID, &owner, &a.Role, &a.FreeCallLimit,
		&a.CallsUsed, &a.CreatedAt, &verified)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	if err != nil {
		return Account{}, err
	}
	a.IsOwner = owner == 1
	a.Role = roleFor(a.IsOwner, a.Role)
	a.EmailVerified = verified.Valid
	return a, nil
}

func (s *Store) ListAccounts(ctx context.Context, tenantID string) ([]Account, error) {
	rows, err := s.queryRows(ctx, `
SELECT id, email, name, tenant_id, is_owner, role, free_call_limit, calls_used, created_at, email_verified_at
FROM accounts WHERE (? = '' OR tenant_id = ?) ORDER BY created_at`, tenantID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Account
	for rows.Next() {
		var a Account
		var owner int
		var verified sql.NullInt64
		if err := rows.Scan(&a.ID, &a.Email, &a.Name, &a.TenantID, &owner, &a.Role,
			&a.FreeCallLimit, &a.CallsUsed, &a.CreatedAt, &verified); err != nil {
			return nil, err
		}
		a.IsOwner = owner == 1
		a.Role = roleFor(a.IsOwner, a.Role)
		a.EmailVerified = verified.Valid
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) UpdateRole(ctx context.Context, accountID, role string) error {
	if !ValidRole(role) || role == RoleOwner {
		return ErrInvalidRole
	}
	result, err := s.exec(ctx, "UPDATE accounts SET role = ? WHERE id = ? AND is_owner = 0", role, accountID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteAccount(ctx context.Context, accountID string) error {
	_, _ = s.exec(ctx, "DELETE FROM sessions WHERE account_id = ?", accountID)
	result, err := s.exec(ctx, "DELETE FROM accounts WHERE id = ? AND is_owner = 0", accountID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
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
