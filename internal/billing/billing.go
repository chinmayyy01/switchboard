package billing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/chinmay/switchboard/internal/metrics"
)

type Rate struct {
	Per1KMinor     int64 `json:"per_1k_minor"`
	PerSecondMinor int64 `json:"per_second_minor"`
}

type Table struct {
	Currency string          `json:"currency"`
	Default  Rate            `json:"default"`
	Models   map[string]Rate `json:"models"`
}

func Load(path string) (*Table, error) {
	if strings.TrimSpace(path) == "" {
		return &Table{Currency: "INR", Models: map[string]Rate{}}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var table Table
	if err := json.Unmarshal(data, &table); err != nil {
		return nil, err
	}
	if table.Currency == "" {
		table.Currency = "INR"
	}
	if table.Models == nil {
		table.Models = map[string]Rate{}
	}
	return &table, nil
}

// Cost returns the price in minor units and whether the model was priced.
func (t *Table) Cost(kind, model string, units int64) (int64, bool) {
	if t == nil || units <= 0 {
		return 0, true
	}
	key := strings.ToLower(kind + ":" + strings.TrimSpace(model))
	rate, ok := t.Models[key]
	if !ok {
		rate, ok = t.Models[kind+":default"]
	}
	if !ok {
		rate = t.Default
	}
	switch kind {
	case "llm_input", "llm_output", "tts_characters":
		return units * rate.Per1KMinor / 1000, rate.Per1KMinor > 0
	case "stt_seconds", "call_seconds":
		return units * rate.PerSecondMinor, rate.PerSecondMinor > 0
	default:
		return 0, false
	}
}

type UsageItem struct {
	Kind  string `json:"kind"`
	Model string `json:"model"`
	Units int64  `json:"units"`
}

type Decision struct {
	Allowed        bool   `json:"allowed"`
	Action         string `json:"action"`
	Currency       string `json:"currency"`
	CapMinor       int64  `json:"cap_minor"`
	SpentMinor     int64  `json:"spent_minor"`
	RemainingMinor int64  `json:"remaining_minor"`
}

type Service struct {
	DB              *sql.DB
	Prices          *Table
	DefaultCapMinor int64
	DefaultAction   string
	Now             func() time.Time
}

func New(db *sql.DB, prices *Table, capMinor int64, action string) *Service {
	if action == "" {
		action = "transfer"
	}
	return &Service{DB: db, Prices: prices, DefaultCapMinor: capMinor, DefaultAction: action, Now: time.Now}
}

func Period(t time.Time) string { return t.Format("2006-01") }

func (s *Service) Check(ctx context.Context, tenantID string) (Decision, error) {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	period := Period(now)
	decision := Decision{Allowed: true, Action: s.DefaultAction, Currency: "INR", CapMinor: s.DefaultCapMinor}
	var action, currency string
	err := s.DB.QueryRowContext(ctx, `
SELECT cap_minor, currency, action FROM tenant_budgets WHERE tenant_id = $1`, tenantID,
	).Scan(&decision.CapMinor, &currency, &action)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return decision, err
	}
	if err == nil {
		decision.Currency = currency
		decision.Action = action
	}
	if s.Prices != nil && s.Prices.Currency != "" {
		decision.Currency = s.Prices.Currency
	}
	if err := s.DB.QueryRowContext(ctx, `
SELECT COALESCE(SUM(cost_minor), 0) FROM tenant_usage WHERE tenant_id = $1 AND period = $2`,
		tenantID, period).Scan(&decision.SpentMinor); err != nil {
		return decision, err
	}
	if decision.CapMinor > 0 {
		decision.RemainingMinor = decision.CapMinor - decision.SpentMinor
		if decision.RemainingMinor <= 0 {
			decision.Allowed = false
			decision.RemainingMinor = 0
		}
	}
	return decision, nil
}

// Record writes the per-call ledger. It is idempotent per call/kind/model so a
// retried shutdown callback cannot double-bill.
func (s *Service) Record(ctx context.Context, tenantID, callID string, items []UsageItem) (int64, error) {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	period := Period(now)
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var total int64
	for _, item := range items {
		if item.Units <= 0 {
			continue
		}
		cost, priced := s.Prices.Cost(item.Kind, item.Model, item.Units)
		if !priced {
			// An unpriced modality is not free. Record the units with a zero
			// cost and let the alert fire so the table gets fixed.
			cost = 0
			metrics.BillingUnpriced()
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO tenant_usage (tenant_id, call_id, period, kind, model, units, cost_minor)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (call_id, kind, model) DO UPDATE SET
    units = EXCLUDED.units,
    cost_minor = EXCLUDED.cost_minor`,
			tenantID, callID, period, item.Kind, item.Model, item.Units, cost); err != nil {
			return 0, err
		}
		total += cost
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return total, nil
}

type UsageLine struct {
	Kind      string `json:"kind"`
	Model     string `json:"model"`
	Units     int64  `json:"units"`
	CostMinor int64  `json:"cost_minor"`
}

type Summary struct {
	Period          string      `json:"period"`
	Currency        string      `json:"currency"`
	Action          string      `json:"action"`
	CapMinor        int64       `json:"cap_minor"`
	SpentMinor      int64       `json:"spent_minor"`
	RemainingMinor  int64       `json:"remaining_minor"`
	DefaultCapMinor int64       `json:"default_cap_minor"`
	Items           []UsageLine `json:"items"`
}

// Summary is the console view of the current period: the cap in force, what has
// been spent, and the per-modality breakdown.
func (s *Service) Summary(ctx context.Context, tenantID string) (Summary, error) {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	out := Summary{
		Period:          Period(now),
		Currency:        "INR",
		Action:          s.DefaultAction,
		CapMinor:        s.DefaultCapMinor,
		DefaultCapMinor: s.DefaultCapMinor,
		Items:           []UsageLine{},
	}
	var action, currency string
	err := s.DB.QueryRowContext(ctx, `
SELECT cap_minor, currency, action FROM tenant_budgets WHERE tenant_id = $1`, tenantID,
	).Scan(&out.CapMinor, &currency, &action)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return out, err
	}
	if err == nil {
		out.Currency = currency
		out.Action = action
	}
	if s.Prices != nil && s.Prices.Currency != "" {
		out.Currency = s.Prices.Currency
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT kind, model, COALESCE(SUM(units), 0), COALESCE(SUM(cost_minor), 0)
FROM tenant_usage
WHERE tenant_id = $1 AND period = $2
GROUP BY kind, model
ORDER BY kind, model`, tenantID, out.Period)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var line UsageLine
		if err := rows.Scan(&line.Kind, &line.Model, &line.Units, &line.CostMinor); err != nil {
			return out, err
		}
		out.SpentMinor += line.CostMinor
		out.Items = append(out.Items, line)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}
	if out.CapMinor > 0 {
		out.RemainingMinor = out.CapMinor - out.SpentMinor
		if out.RemainingMinor < 0 {
			out.RemainingMinor = 0
		}
	}
	return out, nil
}

// SetBudget creates or updates the monthly cap. Zero means unlimited.
func (s *Service) SetBudget(ctx context.Context, tenantID string, capMinor int64, action string) error {
	if capMinor < 0 {
		return errors.New("cap cannot be negative")
	}
	switch action {
	case "":
		action = s.DefaultAction
	case "transfer", "refuse", "degrade":
	default:
		return errors.New("invalid budget action")
	}
	currency := "INR"
	if s.Prices != nil && s.Prices.Currency != "" {
		currency = s.Prices.Currency
	}
	_, err := s.DB.ExecContext(ctx, `
INSERT INTO tenant_budgets (tenant_id, cap_minor, currency, action)
VALUES ($1, $2, $3, $4)
ON CONFLICT (tenant_id) DO UPDATE SET
    cap_minor = EXCLUDED.cap_minor,
    currency = EXCLUDED.currency,
    action = EXCLUDED.action,
    updated_at = NOW()`, tenantID, capMinor, currency, action)
	return err
}
