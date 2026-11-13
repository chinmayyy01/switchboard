package calls

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Recording struct {
	CallID            string     `json:"call_id"`
	TenantID          string     `json:"tenant_id"`
	EgressID          string     `json:"egress_id"`
	Status            string     `json:"status"`
	ObjectKey         string     `json:"object_key,omitempty"`
	Error             string     `json:"error,omitempty"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	EndedAt           *time.Time `json:"ended_at,omitempty"`
	DurationMS        int64      `json:"duration_ms"`
	Channels          int        `json:"channels"`
	ConsentAt         *time.Time `json:"consent_at,omitempty"`
	DisclosureVersion string     `json:"disclosure_version"`
}

// StartRecording records the Egress job only when the caller consented to
// audio recording. The consent check is inside the INSERT, like turns.
func (s *Store) StartRecording(ctx context.Context, rec Recording) (bool, error) {
	if rec.Channels == 0 {
		rec.Channels = 2
	}
	result, err := s.db.ExecContext(ctx, `
INSERT INTO call_recordings
    (call_id, tenant_id, egress_id, status, channels, started_at, consent_at, disclosure_version)
SELECT $1, $2, $3, 'started', $4, NOW(), c.consent_at, c.consent_version
FROM calls c
WHERE c.id = $1 AND c.tenant_id = $2 AND c.recording_consent IS TRUE
ON CONFLICT (call_id) DO UPDATE SET
    egress_id = EXCLUDED.egress_id,
    status = 'started',
    error = '',
    started_at = NOW(),
    updated_at = NOW()`,
		rec.CallID, rec.TenantID, rec.EgressID, rec.Channels)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *Store) FinishRecordingByEgress(ctx context.Context, egressID, status, objectKey, errMsg string, durationMS int64) (int64, error) {
	if egressID == "" {
		return 0, nil
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE call_recordings
SET status = $2,
    object_key = COALESCE(NULLIF($3, ''), object_key),
    error = $4,
    duration_ms = $5,
    ended_at = NOW(),
    updated_at = NOW()
WHERE egress_id = $1`,
		egressID, status, objectKey, errMsg, durationMS)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) FinishRecordingByCall(ctx context.Context, callID, status, errMsg string) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
UPDATE call_recordings
SET status = $2,
    error = $3,
    ended_at = COALESCE(ended_at, NOW()),
    updated_at = NOW()
WHERE call_id = $1`, callID, status, errMsg)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Store) Recording(ctx context.Context, callID, tenantID string) (*Recording, error) {
	query := `
SELECT call_id, tenant_id, egress_id, status, object_key, error,
       started_at, ended_at, duration_ms, channels, consent_at, disclosure_version
FROM call_recordings
WHERE call_id = $1`
	args := []any{callID}
	if tenantID != "" {
		query += " AND tenant_id = $2"
		args = append(args, tenantID)
	}
	return scanRecording(s.db.QueryRowContext(ctx, query, args...))
}

func (s *Store) RecordingByEgress(ctx context.Context, egressID string) (*Recording, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT call_id, tenant_id, egress_id, status, object_key, error,
       started_at, ended_at, duration_ms, channels, consent_at, disclosure_version
FROM call_recordings
WHERE egress_id = $1`, egressID)
	return scanRecording(row)
}

func scanRecording(row *sql.Row) (*Recording, error) {
	var rec Recording
	var started, ended, consent sql.NullTime
	err := row.Scan(
		&rec.CallID, &rec.TenantID, &rec.EgressID, &rec.Status, &rec.ObjectKey, &rec.Error,
		&started, &ended, &rec.DurationMS, &rec.Channels, &consent, &rec.DisclosureVersion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if started.Valid {
		t := started.Time
		rec.StartedAt = &t
	}
	if ended.Valid {
		t := ended.Time
		rec.EndedAt = &t
	}
	if consent.Valid {
		t := consent.Time
		rec.ConsentAt = &t
	}
	return &rec, nil
}

// ExpiredRecordings returns completed recordings whose retention window has
// elapsed and whose object still exists.
func (s *Store) ExpiredRecordings(ctx context.Context, before time.Time) ([]Recording, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT call_id, tenant_id, egress_id, status, object_key, error,
       started_at, ended_at, duration_ms, channels, consent_at, disclosure_version
FROM call_recordings
WHERE ended_at IS NOT NULL
  AND ended_at < $1
  AND status IN ('completed', 'failed', 'aborted')
  AND object_key <> ''
ORDER BY ended_at`, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Recording
	for rows.Next() {
		var rec Recording
		var started, ended, consent sql.NullTime
		if err := rows.Scan(
			&rec.CallID, &rec.TenantID, &rec.EgressID, &rec.Status, &rec.ObjectKey, &rec.Error,
			&started, &ended, &rec.DurationMS, &rec.Channels, &consent, &rec.DisclosureVersion,
		); err != nil {
			return nil, err
		}
		if started.Valid {
			t := started.Time
			rec.StartedAt = &t
		}
		if ended.Valid {
			t := ended.Time
			rec.EndedAt = &t
		}
		if consent.Valid {
			t := consent.Time
			rec.ConsentAt = &t
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) MarkRecordingDeleted(ctx context.Context, callID string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE call_recordings
SET status = 'deleted', object_key = '', error = '', updated_at = NOW()
WHERE call_id = $1`, callID)
	return err
}

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
