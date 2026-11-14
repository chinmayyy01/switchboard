package calls

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// DisclosureVersion identifies the consent wording the caller heard. Bump it
// whenever the disclosure text changes so stored consent evidence stays
// interpretable.
const DisclosureVersion = "2026-11-14"

type Call struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	Direction    string `json:"direction"`
	FromNumber   string `json:"from_number"`
	ToNumber     string `json:"to_number"`
	FromRaw      string `json:"from_number_raw,omitempty"`
	ToRaw        string `json:"to_number_raw,omitempty"`
	SIPCallID    string `json:"sip_call_id,omitempty"`
	Source       string `json:"source,omitempty"`
	AgentVersion string `json:"agent_version,omitempty"`
	RoomName     string `json:"room_name"`
	Outcome      string `json:"outcome"`
	Escalated    bool   `json:"escalated"`
	Transcript   *bool  `json:"transcript_consent,omitempty"`
	ManagerPhone string `json:"manager_number,omitempty"`
}

type Turn struct {
	CallID    string `json:"call_id"`
	TenantID  string `json:"tenant_id"`
	Seq       int    `json:"seq"`
	Role      string `json:"role"`
	Text      string `json:"text"`
	LatencyMS int    `json:"latency_ms,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
}

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Start(ctx context.Context, call Call) error {
	if call.Direction == "" {
		call.Direction = "inbound"
	}
	from, fromRaw := NormalizeCallerNumber(call.FromNumber)
	if fromRaw == "" {
		fromRaw = call.FromRaw
	}
	to, toRaw := NormalizeCallerNumber(call.ToNumber)
	if toRaw == "" {
		toRaw = call.ToRaw
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO calls (id, tenant_id, direction, from_number, to_number, from_number_raw, to_number_raw,
                   sip_call_id, source, room_name, manager_number, agent_version)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (id) DO UPDATE SET
    from_number = COALESCE(NULLIF(EXCLUDED.from_number, ''), calls.from_number),
    to_number = COALESCE(NULLIF(EXCLUDED.to_number, ''), calls.to_number),
    from_number_raw = COALESCE(NULLIF(EXCLUDED.from_number_raw, ''), calls.from_number_raw),
    to_number_raw = COALESCE(NULLIF(EXCLUDED.to_number_raw, ''), calls.to_number_raw),
    sip_call_id = COALESCE(NULLIF(EXCLUDED.sip_call_id, ''), calls.sip_call_id),
    source = COALESCE(NULLIF(EXCLUDED.source, ''), calls.source),
    room_name = COALESCE(NULLIF(EXCLUDED.room_name, ''), calls.room_name),
    manager_number = COALESCE(NULLIF(EXCLUDED.manager_number, ''), calls.manager_number),
    agent_version = COALESCE(NULLIF(EXCLUDED.agent_version, ''), calls.agent_version)`,
		call.ID, call.TenantID, call.Direction, from, to, fromRaw, toRaw,
		call.SIPCallID, call.Source, call.RoomName, call.ManagerPhone, call.AgentVersion)
	return err
}

// SetConsent stores the caller's transcript decision. No audio is recorded, so
// there is no recording consent to keep.
func (s *Store) SetConsent(ctx context.Context, id string, transcript bool, language string) error {
	if language == "" {
		language = "en"
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE calls
SET transcript_consent = $2,
    consent_at = NOW(),
    consent_version = $3,
    consent_language = $4
WHERE id = $1`, id, transcript, DisclosureVersion, language)
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

func (s *Store) Escalate(ctx context.Context, id, reason string) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE calls SET escalated = TRUE, escalation_reason = $2 WHERE id = $1`, id, reason)
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

func (s *Store) End(ctx context.Context, id, outcome string) (bool, error) {
	return s.EndWithReason(ctx, id, outcome, "")
}

func (s *Store) EndWithReason(ctx context.Context, id, outcome, reason string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
UPDATE calls SET ended_at = NOW(), outcome = $2, end_reason = $3
WHERE id = $1 AND ended_at IS NULL`, id, outcome, reason)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM calls WHERE id = $1)", id).Scan(&exists); err != nil {
		return false, err
	}
	if !exists {
		return false, ErrNotFound
	}
	return false, nil
}

// EndByRoom closes the open call for a LiveKit room. It is idempotent: a
// webhook retry, or a webhook arriving after the agent already ended the call,
// is a no-op rather than an error.
func (s *Store) EndByRoom(ctx context.Context, room, outcome, reason string) (int64, error) {
	if room == "" {
		return 0, nil
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE calls
SET ended_at = NOW(), outcome = $2, end_reason = $3
WHERE room_name = $1 AND ended_at IS NULL`, room, outcome, reason)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// SetCallerByRoom fills in caller identity from SIP participant attributes.
// Empty values never overwrite an existing number.
func (s *Store) SetCallerByRoom(ctx context.Context, room, from, to, sipCallID string) (int64, error) {
	if room == "" {
		return 0, nil
	}
	fromE164, fromRaw := NormalizeCallerNumber(from)
	toE164, toRaw := NormalizeCallerNumber(to)
	result, err := s.db.ExecContext(ctx, `
UPDATE calls
SET from_number = COALESCE(NULLIF($2, ''), from_number),
    from_number_raw = COALESCE(NULLIF($3, ''), from_number_raw),
    to_number = COALESCE(NULLIF($4, ''), to_number),
    to_number_raw = COALESCE(NULLIF($5, ''), to_number_raw),
    sip_call_id = COALESCE(NULLIF($6, ''), sip_call_id),
    source = CASE
        WHEN COALESCE(NULLIF($2, ''), NULLIF($4, '')) IS NOT NULL THEN 'sip'
        ELSE source
    END
WHERE room_name = $1`, room, fromE164, fromRaw, toE164, toRaw, sipCallID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) TranscriptConsent(ctx context.Context, id string) (*bool, error) {
	var allowed sql.NullBool
	err := s.db.QueryRowContext(ctx,
		"SELECT transcript_consent FROM calls WHERE id = $1", id).Scan(&allowed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !allowed.Valid {
		return nil, nil
	}
	value := allowed.Bool
	return &value, nil
}

// AppendTurn persists one transcript turn only when the call has recorded
// transcript consent. The consent check lives in the INSERT so a bug in the
// caller cannot bypass it. Duplicate sequence numbers are ignored, which makes
// agent retries safe.
func (s *Store) AppendTurn(ctx context.Context, turn Turn) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO call_turns (call_id, tenant_id, seq, role, text, latency_ms, tool_name)
SELECT $1, $2, $3, $4, $5, $6, $7
WHERE EXISTS (
    SELECT 1 FROM calls
    WHERE id = $1 AND tenant_id = $2 AND transcript_consent IS TRUE
)
ON CONFLICT (call_id, seq) DO NOTHING`,
		turn.CallID, turn.TenantID, turn.Seq, turn.Role, turn.Text, turn.LatencyMS, turn.ToolName)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ReapStale closes calls that never received an end event. A crashed agent,
// a dropped SIP leg, or a missed webhook must not leave a call open forever.
func (s *Store) ReapStale(ctx context.Context, olderThan time.Duration, tenantID string) (int64, error) {
	query := `
UPDATE calls
SET ended_at = NOW(), outcome = 'abandoned', end_reason = 'reaper'
WHERE ended_at IS NULL AND started_at < NOW() - make_interval(secs => $1)`
	args := []any{olderThan.Seconds()}
	if tenantID != "" {
		query += " AND tenant_id = $2"
		args = append(args, tenantID)
	}
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

var ErrNotFound = errors.New("call not found")

type Draft struct {
	Service string
	Address string
	Phone   string
}

// SaveDraftField stores a validated booking field for the call. The booking
// confirmation reads these values instead of trusting the model to repeat them,
// which prevents invented phone numbers and addresses.
func (s *Store) SaveDraftField(ctx context.Context, tenantID, callID, field, value string) error {
	switch field {
	case "service", "address", "phone":
	default:
		return ErrInvalidField
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO call_drafts (call_id, tenant_id, `+field+`, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (call_id) DO UPDATE SET
    tenant_id = EXCLUDED.tenant_id,
    `+field+` = EXCLUDED.`+field+`,
    updated_at = NOW()`, callID, tenantID, value)
	return err
}

func (s *Store) Draft(ctx context.Context, callID string) (Draft, error) {
	var draft Draft
	err := s.db.QueryRowContext(ctx,
		"SELECT service, address, phone FROM call_drafts WHERE call_id = $1", callID,
	).Scan(&draft.Service, &draft.Address, &draft.Phone)
	if errors.Is(err, sql.ErrNoRows) {
		return Draft{}, nil
	}
	return draft, err
}

var ErrInvalidField = errors.New("invalid draft field")

// DeleteTranscriptsBefore enforces transcript retention. Calls and consent
// evidence remain; only the turn text is removed.
func (s *Store) DeleteTranscriptsBefore(ctx context.Context, before time.Time) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
DELETE FROM call_turns WHERE created_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
