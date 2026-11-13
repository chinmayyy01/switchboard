package calls

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chinmay/switchboard/internal/jobs"
)

func TestRecordingLifecycleRequiresConsent(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := jobs.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	tenantID := "rec-test-" + time.Now().Format("150405.000000")
	callID := tenantID + "-call"
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM calls WHERE tenant_id = $1", tenantID)
		db.Close()
	})
	store := New(db)
	if err := store.Start(ctx, Call{ID: callID, TenantID: tenantID, RoomName: tenantID + "-room"}); err != nil {
		t.Fatal(err)
	}

	// No consent: no recording row.
	ok, err := store.StartRecording(ctx, Recording{CallID: callID, TenantID: tenantID, EgressID: "egress-0"})
	if err != nil || ok {
		t.Fatalf("recording without consent ok=%v err=%v", ok, err)
	}

	if err := store.SetConsent(ctx, callID, true, true, "en"); err != nil {
		t.Fatal(err)
	}
	ok, err = store.StartRecording(ctx, Recording{CallID: callID, TenantID: tenantID, EgressID: "egress-1"})
	if err != nil || !ok {
		t.Fatalf("recording with consent ok=%v err=%v", ok, err)
	}
	n, err := store.FinishRecordingByEgress(ctx, "egress-1", "completed", "recordings/call.ogg", "", 5000)
	if err != nil || n != 1 {
		t.Fatalf("finish n=%d err=%v", n, err)
	}
	rec, err := store.Recording(ctx, callID, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "completed" || rec.ObjectKey != "recordings/call.ogg" || rec.DurationMS != 5000 {
		t.Fatalf("recording = %+v", rec)
	}

	expired, err := store.ExpiredRecordings(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, candidate := range expired {
		if candidate.CallID == callID {
			found = true
		}
	}
	if !found {
		t.Fatal("completed recording was not returned by retention query")
	}

	if err := store.MarkRecordingDeleted(ctx, callID); err != nil {
		t.Fatal(err)
	}
	rec, err = store.Recording(ctx, callID, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "deleted" || rec.ObjectKey != "" {
		t.Fatalf("deleted recording = %+v", rec)
	}
}
