package jobs

import (
	"context"
	"testing"
)

func TestRegistryIsolation(t *testing.T) {
	r := NewRegistry("")
	a := r.For("a")
	b := r.For("b")

	if _, err := a.BookContext(context.Background(), BookingRequest{
		IdempotencyKey: "call-a-book-1",
		Service:        "leak repair",
		Date:           "2026-10-30",
		Window:         "morning",
		Address:        "1 A St",
		Phone:          "5550001111",
	}); err != nil {
		t.Fatal(err)
	}
	if found, err := a.ByPhoneContext(context.Background(), "5550001111"); err != nil || len(found) != 1 {
		t.Fatal("job missing in tenant a")
	}
	if found, err := b.ByPhoneContext(context.Background(), "5550001111"); err != nil || len(found) != 0 {
		t.Fatal("job leaked into tenant b")
	}
	if r.For("a") != a {
		t.Fatal("For should return the same store")
	}
}
