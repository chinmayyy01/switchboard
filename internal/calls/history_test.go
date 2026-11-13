package calls

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chinmay/switchboard/internal/jobs"
)

func TestCallHistoryListLiveAndLabels(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := jobs.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	tenantID := "history-test-" + time.Now().Format("150405.000000")
	callID := tenantID + "-call"
	room := tenantID + "-room"
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM calls WHERE tenant_id = $1", tenantID)
		db.Close()
	})
	store := New(db)
	if err := store.Start(ctx, Call{ID: callID, TenantID: tenantID, RoomName: room, Source: "sip"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetConsent(ctx, callID, false, true, "en"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendTurn(ctx, Turn{CallID: callID, TenantID: tenantID, Seq: 1, Role: "user", Text: "my sink is leaking"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendTurn(ctx, Turn{CallID: callID, TenantID: tenantID, Seq: 2, Role: "assistant", Text: "I can help with that."}); err != nil {
		t.Fatal(err)
	}

	live, err := store.Live(ctx, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 1 || live[0].LastText != "I can help with that." || len(live[0].Transcript) != 2 {
		t.Fatalf("live = %+v", live)
	}
	if _, err := store.EndByRoom(ctx, room, "completed", "room_finished"); err != nil {
		t.Fatal(err)
	}

	records, err := store.List(ctx, tenantID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].Turns != 2 || records[0].Summary != "my sink is leaking" {
		t.Fatalf("records = %+v", records)
	}
	if records[0].RoomName != room || records[0].Source != "sip" {
		t.Fatalf("record identity = %+v", records[0])
	}

	if err := store.SetLabel(ctx, callID, true); err != nil {
		t.Fatal(err)
	}
	labeled, err := store.Labeled(ctx, tenantID)
	if err != nil || len(labeled) != 1 || !labeled[0].Actual {
		t.Fatalf("labeled = %+v err=%v", labeled, err)
	}
	rec, err := store.CallByID(ctx, callID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Label == nil || !*rec.Label {
		t.Fatalf("label = %v", rec.Label)
	}

	if _, err := store.EndByRoom(ctx, room, "completed", "room_finished"); err != nil {
		t.Fatal(err)
	}
	live, err = store.Live(ctx, tenantID)
	if err != nil || len(live) != 0 {
		t.Fatalf("live after end = %+v err=%v", live, err)
	}
}
