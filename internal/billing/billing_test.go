package billing

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chinmay/switchboard/internal/jobs"
)

func TestCost(t *testing.T) {
	table := &Table{
		Currency: "INR",
		Default:  Rate{Per1KMinor: 10, PerSecondMinor: 1},
		Models: map[string]Rate{
			"llm_input:acme/model": {Per1KMinor: 1000},
		},
	}
	if got, ok := table.Cost("llm_input", "acme/model", 1500); !ok || got != 1500 {
		t.Fatalf("cost = %d ok=%v", got, ok)
	}
	if got, ok := table.Cost("stt_seconds", "unknown", 10); !ok || got != 10 {
		t.Fatalf("default cost = %d ok=%v", got, ok)
	}
}

func TestCheckAndRecordIsIdempotent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := jobs.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	tenantID := "billing-test-" + time.Now().Format("150405.000000")
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM tenant_usage WHERE tenant_id = $1", tenantID)
		db.ExecContext(context.Background(), "DELETE FROM tenant_budgets WHERE tenant_id = $1", tenantID)
		db.Close()
	})
	if _, err := db.ExecContext(ctx,
		"INSERT INTO tenant_budgets (tenant_id, cap_minor, currency, action) VALUES ($1, 500, 'INR', 'transfer')",
		tenantID); err != nil {
		t.Fatal(err)
	}
	prices := &Table{
		Currency: "INR",
		Models: map[string]Rate{
			"llm_input:acme/model": {Per1KMinor: 500},
		},
	}
	svc := New(db, prices, 0, "transfer")
	svc.Now = func() time.Time { return time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC) }

	decision, err := svc.Check(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed || decision.CapMinor != 500 {
		t.Fatalf("initial decision = %+v", decision)
	}
	cost, err := svc.Record(ctx, tenantID, "call-1", []UsageItem{
		{Kind: "llm_input", Model: "acme/model", Units: 1000},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cost != 500 {
		t.Fatalf("cost = %d", cost)
	}
	// A retried shutdown callback must not double-bill.
	if _, err := svc.Record(ctx, tenantID, "call-1", []UsageItem{
		{Kind: "llm_input", Model: "acme/model", Units: 1000},
	}); err != nil {
		t.Fatal(err)
	}
	decision, err = svc.Check(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed || decision.SpentMinor != 500 || decision.RemainingMinor != 0 {
		t.Fatalf("exhausted decision = %+v", decision)
	}
}
