package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/chinmay/switchboard/internal/auth"
	"github.com/chinmay/switchboard/internal/hub"
	"github.com/chinmay/switchboard/internal/jobs"
	"github.com/chinmay/switchboard/internal/store"
	"github.com/chinmay/switchboard/internal/tenant"
)

func TestOneCallPerAccount(t *testing.T) {
	t.Setenv("AUTH_DISABLED", "false")
	authStore, err := auth.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer authStore.Close()
	account, err := authStore.CreateAccount(context.Background(), "owner@test", "secret", "rivera", false)
	if err != nil {
		t.Fatal(err)
	}
	token, err := authStore.CreateSession(context.Background(), account.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	limiter := auth.NewCallLimiter()
	server := testServer(t, authStore, limiter)
	defer server.Close()

	first := dial(t, server.URL, token)
	defer first.Close(websocket.StatusNormalClosure, "")
	if _, _, err := websocket.Dial(context.Background(), wsURL(server.URL), dialOptions(token)); err == nil {
		t.Fatal("second call should be rejected")
	}

	first.Close(websocket.StatusNormalClosure, "")
	waitFor(t, func() bool { return !limiter.Active(account.ID) })
	third := dial(t, server.URL, token)
	third.Close(websocket.StatusNormalClosure, "")
	waitFor(t, func() bool { return !limiter.Active(account.ID) })
	if _, _, err := websocket.Dial(context.Background(), wsURL(server.URL), dialOptions(token)); err == nil {
		t.Fatal("third call should hit the free-call quota")
	}
}

func TestOwnerHasNoCallLimit(t *testing.T) {
	t.Setenv("AUTH_DISABLED", "false")
	authStore, err := auth.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer authStore.Close()
	account, err := authStore.CreateAccount(context.Background(), "owner@test", "secret", "rivera", true)
	if err != nil {
		t.Fatal(err)
	}
	token, err := authStore.CreateSession(context.Background(), account.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	server := testServer(t, authStore, auth.NewCallLimiter())
	defer server.Close()

	first := dial(t, server.URL, token)
	defer first.Close(websocket.StatusNormalClosure, "")
	second := dial(t, server.URL, token)
	defer second.Close(websocket.StatusNormalClosure, "")
}

func testServer(t *testing.T, authStore *auth.Store, limiter *auth.CallLimiter) *httptest.Server {
	t.Helper()
	h := hub.New()
	st := store.New(t.TempDir(), 100)
	reg := NewBridgeRegistry()
	jobsReg := jobs.NewRegistry("")
	tenants := tenant.Default()
	handler := Handler(h, st, reg, jobsReg, tenants, authStore, limiter, context.Background())
	return httptest.NewServer(auth.RequireAccount(authStore, handler))
}

func dial(t *testing.T, baseURL, token string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, wsURL(baseURL), dialOptions(token))
	if err != nil {
		t.Fatal(err)
	}
	return conn
}

func dialOptions(token string) *websocket.DialOptions {
	header := http.Header{}
	header.Set("Cookie", auth.CookieName+"="+token)
	return &websocket.DialOptions{HTTPHeader: header}
}

func wsURL(baseURL string) string {
	return "ws" + strings.TrimPrefix(baseURL, "http") + "/ws?tenant=rivera"
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met")
}
