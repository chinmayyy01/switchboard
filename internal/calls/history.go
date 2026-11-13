package calls

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Record struct {
	ID         string    `json:"id"`
	Tenant     string    `json:"tenant"`
	Lang       string    `json:"lang"`
	Turns      int       `json:"turns"`
	Escalated  bool      `json:"escalated"`
	Reason     string    `json:"reason"`
	Transcript []Turn    `json:"transcript,omitempty"`
	Summary    string    `json:"summary,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
	Label      *bool     `json:"label,omitempty"`
	Source     string    `json:"source,omitempty"`
	FromNumber string    `json:"from_number,omitempty"`
	RoomName   string    `json:"room_name,omitempty"`
}

type LiveCall struct {
	ID         string    `json:"id"`
	Tenant     string    `json:"tenant"`
	Lang       string    `json:"lang"`
	State      string    `json:"state"`
	LastText   string    `json:"last_text"`
	Escalated  bool      `json:"escalated"`
	Reason     string    `json:"reason"`
	TakenOver  bool      `json:"taken_over"`
	Transcript []Turn    `json:"transcript"`
	StartedAt  time.Time `json:"started_at"`
}

type LabeledCall struct {
	ID        string
	Predicted bool
	Actual    bool
}

const callSelect = `
SELECT c.id, c.tenant_id, COALESCE(NULLIF(c.consent_language, ''), 'en'),
       c.escalated, COALESCE(c.escalation_reason, ''),
       c.started_at, c.ended_at,
       COALESCE(c.source, ''), COALESCE(c.from_number, ''), COALESCE(c.room_name, ''),
       l.label,
       COALESCE((
           SELECT json_agg(json_build_object('role', t.role, 'text', t.text) ORDER BY t.seq)
           FROM call_turns t WHERE t.call_id = c.id
       ), '[]'::json)`

func (s *Store) List(ctx context.Context, tenantID string, limit int) ([]Record, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, callSelect+`
FROM calls c
LEFT JOIN call_labels l ON l.call_id = c.id
WHERE c.ended_at IS NOT NULL AND ($1 = '' OR c.tenant_id = $1)
ORDER BY c.started_at DESC
LIMIT $2`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) Live(ctx context.Context, tenantID string) ([]LiveCall, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT c.id, c.tenant_id, COALESCE(NULLIF(c.consent_language, ''), 'en'),
       c.escalated, COALESCE(c.escalation_reason, ''), c.started_at,
       COALESCE((
           SELECT json_agg(json_build_object('role', t.role, 'text', t.text) ORDER BY t.seq)
           FROM call_turns t WHERE t.call_id = c.id
       ), '[]'::json)
FROM calls c
WHERE c.ended_at IS NULL AND ($1 = '' OR c.tenant_id = $1)
ORDER BY c.started_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LiveCall
	for rows.Next() {
		var call LiveCall
		var transcript []byte
		if err := rows.Scan(&call.ID, &call.Tenant, &call.Lang, &call.Escalated,
			&call.Reason, &call.StartedAt, &transcript); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(transcript, &call.Transcript)
		call.State = "live"
		if call.Escalated {
			call.State = "escalated"
		}
		if n := len(call.Transcript); n > 0 {
			call.LastText = call.Transcript[n-1].Text
		}
		out = append(out, call)
	}
	return out, rows.Err()
}

func scanRecord(rows *sql.Rows) (Record, error) {
	var rec Record
	var ended sql.NullTime
	var label sql.NullBool
	var transcript []byte
	if err := rows.Scan(&rec.ID, &rec.Tenant, &rec.Lang, &rec.Escalated, &rec.Reason,
		&rec.StartedAt, &ended, &rec.Source, &rec.FromNumber, &rec.RoomName, &label, &transcript); err != nil {
		return Record{}, err
	}
	if ended.Valid {
		rec.EndedAt = ended.Time
	}
	if label.Valid {
		value := label.Bool
		rec.Label = &value
	}
	_ = json.Unmarshal(transcript, &rec.Transcript)
	rec.Turns = len(rec.Transcript)
	for _, turn := range rec.Transcript {
		if turn.Role == "user" && strings.TrimSpace(turn.Text) != "" {
			rec.Summary = clipText(turn.Text, 120)
			break
		}
	}
	if rec.Escalated && rec.Summary == "" {
		rec.Summary = "Escalated to a human: " + rec.Reason + "."
	}
	return rec, nil
}

func (s *Store) SetLabel(ctx context.Context, callID string, label bool) error {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO call_labels (call_id, tenant_id, label)
SELECT id, tenant_id, $2 FROM calls WHERE id = $1
ON CONFLICT (call_id) DO UPDATE SET label = EXCLUDED.label, updated_at = NOW()`,
		callID, label)
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

func (s *Store) Labeled(ctx context.Context, tenantID string) ([]LabeledCall, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT c.id, c.escalated, l.label
FROM call_labels l JOIN calls c ON c.id = l.call_id
WHERE ($1 = '' OR c.tenant_id = $1)
ORDER BY l.updated_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LabeledCall
	for rows.Next() {
		var item LabeledCall
		if err := rows.Scan(&item.ID, &item.Predicted, &item.Actual); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) CallByID(ctx context.Context, id string) (Record, error) {
	row := s.db.QueryRowContext(ctx, callSelect+`
FROM calls c
LEFT JOIN call_labels l ON l.call_id = c.id
WHERE c.id = $1`, id)
	var rec Record
	var ended sql.NullTime
	var label sql.NullBool
	var transcript []byte
	if err := row.Scan(&rec.ID, &rec.Tenant, &rec.Lang, &rec.Escalated, &rec.Reason,
		&rec.StartedAt, &ended, &rec.Source, &rec.FromNumber, &rec.RoomName, &label, &transcript); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Record{}, ErrNotFound
		}
		return Record{}, err
	}
	if ended.Valid {
		rec.EndedAt = ended.Time
	}
	if label.Valid {
		value := label.Bool
		rec.Label = &value
	}
	_ = json.Unmarshal(transcript, &rec.Transcript)
	rec.Turns = len(rec.Transcript)
	return rec, nil
}

func clipText(s string, n int) string {
	s = strings.TrimSpace(strings.Join(strings.Fields(s), " "))
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
