package hub

import (
	"testing"
	"time"
)

func TestHubPublishAndSubscribe(t *testing.T) {
	h := New()

	id, ch := h.Subscribe()
	if snap := <-ch; len(snap) != 0 {
		t.Fatalf("expected empty initial snapshot, got %d", len(snap))
	}

	h.Register("call-1", "rivera")
	select {
	case snap := <-ch:
		if len(snap) != 1 || snap[0].ID != "call-1" {
			t.Fatalf("bad snapshot: %+v", snap)
		}
	case <-time.After(time.Second):
		t.Fatal("no snapshot after Register")
	}

	h.SetText("call-1", "hello")
	select {
	case snap := <-ch:
		if len(snap) != 1 || snap[0].LastText != "hello" {
			t.Fatalf("bad snapshot: %+v", snap)
		}
	case <-time.After(time.Second):
		t.Fatal("no snapshot after SetText")
	}

	h.Unregister("call-1")
	select {
	case snap := <-ch:
		if len(snap) != 0 {
			t.Fatalf("expected empty after Unregister, got %d", len(snap))
		}
	case <-time.After(time.Second):
		t.Fatal("no snapshot after Unregister")
	}

	h.Unsubscribe(id)
}
