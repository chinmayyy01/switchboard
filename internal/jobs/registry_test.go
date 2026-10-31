package jobs

import "testing"

func TestRegistryIsolation(t *testing.T) {
	r := NewRegistry()
	a := r.For("a")
	b := r.For("b")

	if _, err := a.Book("leak repair", "2026-10-30", "morning", "1 A St", "5550001111"); err != nil {
		t.Fatal(err)
	}
	if len(a.ByPhone("5550001111")) != 1 {
		t.Fatal("job missing in tenant a")
	}
	if len(b.ByPhone("5550001111")) != 0 {
		t.Fatal("job leaked into tenant b")
	}
	if r.For("a") != a {
		t.Fatal("For should return the same store")
	}
}
