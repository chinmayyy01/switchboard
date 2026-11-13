package calls

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chinmay/switchboard/internal/jobs"
)

func TestCallerIdConsentTurnsAndReaper(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := jobs.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	tenantID := "calls-test-" + time.Now().Format("150405.000000")
	callID := tenantID + "-call-1"
	room := tenantID + "-room-1"
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM calls WHERE tenant_id = $1", tenantID)
		db.Close()
	})
	store := New(db)

	// A carrier-formatted number is normalized, but the raw value is kept.
	if err := store.Start(ctx, Call{
		ID: callID, TenantID: tenantID, RoomName: room,
		FromNumber: "09876543210", ToNumber: "+91 80 1234 5678",
	}); err != nil {
		t.Fatal(err)
	}
	var from, fromRaw, to string
	if err := db.QueryRowContext(ctx,
		"SELECT from_number, from_number_raw, to_number FROM calls WHERE id = $1", callID,
	).Scan(&from, &fromRaw, &to); err != nil {
		t.Fatal(err)
	}
	if from != "+919876543210" || fromRaw != "09876543210" || to != "+918012345678" {
		t.Fatalf("caller id = %q raw=%q to=%q", from, fromRaw, to)
	}

	// A retry with no caller number must not erase the known one.
	if err := store.Start(ctx, Call{ID: callID, TenantID: tenantID, RoomName: room}); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT from_number FROM calls WHERE id = $1", callID).Scan(&from); err != nil {
		t.Fatal(err)
	}
	if from != "+919876543210" {
		t.Fatalf("retry erased caller id: %q", from)
	}

	// No consent yet: turns are refused.
	recorded, err := store.AppendTurn(ctx, Turn{
		CallID: callID, TenantID: tenantID, Seq: 1, Role: "user", Text: "hello",
	})
	if err != nil || recorded {
		t.Fatalf("turn before consent recorded=%v err=%v", recorded, err)
	}

	// Audio declined, transcript allowed.
	if err := store.SetConsent(ctx, callID, false, true, "en"); err != nil {
		t.Fatal(err)
	}
	recording, err := store.Consent(ctx, callID)
	if err != nil || recording == nil || *recording {
		t.Fatalf("recording consent = %v err=%v", recording, err)
	}
	transcript, err := store.TranscriptConsent(ctx, callID)
	if err != nil || transcript == nil || !*transcript {
		t.Fatalf("transcript consent = %v err=%v", transcript, err)
	}
	recorded, err = store.AppendTurn(ctx, Turn{
		CallID: callID, TenantID: tenantID, Seq: 1, Role: "user", Text: "hello",
	})
	if err != nil || !recorded {
		t.Fatalf("turn after consent recorded=%v err=%v", recorded, err)
	}
	// Duplicate sequence numbers are idempotent.
	recorded, err = store.AppendTurn(ctx, Turn{
		CallID: callID, TenantID: tenantID, Seq: 1, Role: "user", Text: "hello",
	})
	if err != nil || recorded {
		t.Fatalf("duplicate turn recorded=%v err=%v", recorded, err)
	}

	// Webhook backfill fills the caller ID from SIP attributes.
	n, err := store.SetCallerByRoom(ctx, room, "9876543211", "", "sip-call-1")
	if err != nil || n != 1 {
		t.Fatalf("SetCallerByRoom n=%d err=%v", n, err)
	}
	var source, sipCallID string
	if err := db.QueryRowContext(ctx,
		"SELECT from_number, source, sip_call_id FROM calls WHERE id = $1", callID,
	).Scan(&from, &source, &sipCallID); err != nil {
		t.Fatal(err)
	}
	if from != "+919876543211" || source != "sip" || sipCallID != "sip-call-1" {
		t.Fatalf("backfill from=%q source=%q sip=%q", from, source, sipCallID)
	}

	// Closing by room is idempotent.
	n, err = store.EndByRoom(ctx, room, "completed", "room_finished")
	if err != nil || n != 1 {
		t.Fatalf("EndByRoom n=%d err=%v", n, err)
	}
	n, err = store.EndByRoom(ctx, room, "completed", "room_finished")
	if err != nil || n != 0 {
		t.Fatalf("EndByRoom retry n=%d err=%v", n, err)
	}

	// Stale calls are reaped, scoped to this tenant so a shared dev database
	// is not disturbed.
	staleID := tenantID + "-call-2"
	if err := store.Start(ctx, Call{ID: staleID, TenantID: tenantID, RoomName: tenantID + "-room-2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx,
		"UPDATE calls SET started_at = NOW() - interval '3 hours' WHERE id = $1", staleID); err != nil {
		t.Fatal(err)
	}
	n, err = store.ReapStale(ctx, 2*time.Hour, tenantID)
	if err != nil || n != 1 {
		t.Fatalf("ReapStale n=%d err=%v", n, err)
	}
	var outcome, reason string
	var endedAt time.Time
	if err := db.QueryRowContext(ctx,
		"SELECT outcome, end_reason, ended_at FROM calls WHERE id = $1", staleID,
	).Scan(&outcome, &reason, &endedAt); err != nil {
		t.Fatal(err)
	}
	if outcome != "abandoned" || reason != "reaper" || endedAt.IsZero() {
		t.Fatalf("reaped call outcome=%q reason=%q ended=%v", outcome, reason, endedAt)
	}
}
