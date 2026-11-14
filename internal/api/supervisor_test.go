package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/chinmay/switchboard/internal/auth"
	"github.com/chinmay/switchboard/internal/calls"
	"github.com/chinmay/switchboard/internal/jobs"
)

func TestSupervisorToken(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := jobs.OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	tenantID := "sup-test-" + time.Now().Format("150405.000000")
	callID := tenantID + "-call"
	room := tenantID + "-room"
	t.Cleanup(func() {
		db.ExecContext(context.Background(), "DELETE FROM calls WHERE tenant_id = $1", tenantID)
		db.Close()
	})
	store := calls.New(db)
	if err := store.Start(ctx, calls.Call{ID: callID, TenantID: tenantID, RoomName: room}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIVEKIT_API_KEY", "key")
	t.Setenv("LIVEKIT_API_SECRET", "secret")
	t.Setenv("LIVEKIT_URL", "wss://example.livekit.cloud")

	req := httptest.NewRequest(http.MethodGet, "/api/calls/"+callID+"/supervisor-token", nil)
	req.SetPathValue("id", callID)
	req = req.WithContext(auth.WithAccount(req.Context(), auth.Account{ID: "sup", IsOwner: true}))
	rec := httptest.NewRecorder()
	SupervisorToken(store)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		URL   string `json:"url"`
		Token string `json:"token"`
		Room  string `json:"room"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Token == "" || body.Room != room || body.URL != "wss://example.livekit.cloud" {
		t.Fatalf("body = %+v", body)
	}
}
