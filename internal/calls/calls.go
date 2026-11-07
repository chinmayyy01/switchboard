package calls

import (
	"context"
	"database/sql"
	"errors"
)

type Call struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	Direction    string `json:"direction"`
	FromNumber   string `json:"from_number"`
	ToNumber     string `json:"to_number"`
	RoomName     string `json:"room_name"`
	Outcome      string `json:"outcome"`
	Escalated    bool   `json:"escalated"`
	Consent      *bool  `json:"recording_consent,omitempty"`
	ManagerPhone string `json:"manager_number,omitempty"`
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
	_, err := s.db.ExecContext(ctx, `
INSERT INTO calls (id, tenant_id, direction, from_number, to_number, room_name, manager_number)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (id) DO UPDATE SET
    from_number = EXCLUDED.from_number,
    to_number = EXCLUDED.to_number,
    room_name = EXCLUDED.room_name,
    manager_number = EXCLUDED.manager_number`,
		call.ID, call.TenantID, call.Direction, call.FromNumber, call.ToNumber, call.RoomName, call.ManagerPhone)
	return err
}

func (s *Store) SetConsent(ctx context.Context, id string, allowed bool) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE calls SET recording_consent = $2, consent_at = NOW() WHERE id = $1`, id, allowed)
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

func (s *Store) End(ctx context.Context, id, outcome string) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE calls SET ended_at = NOW(), outcome = $2 WHERE id = $1`, id, outcome)
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

func (s *Store) Consent(ctx context.Context, id string) (*bool, error) {
	var allowed sql.NullBool
	err := s.db.QueryRowContext(ctx,
		"SELECT recording_consent FROM calls WHERE id = $1", id).Scan(&allowed)
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
