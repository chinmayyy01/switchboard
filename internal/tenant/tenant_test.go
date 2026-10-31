package tenant

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndGet(t *testing.T) {
	dir := t.TempDir()
	write := func(id, name string) {
		if err := os.MkdirAll(filepath.Join(dir, id), 0o755); err != nil {
			t.Fatal(err)
		}
		body := `{"id":"` + id + `","name":"` + name + `"}`
		if err := os.WriteFile(filepath.Join(dir, id, "config.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a", "A Co")
	write("b", "B Co")

	reg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Get("a"); !ok {
		t.Fatal("missing a")
	}
	if _, ok := reg.Get("b"); !ok {
		t.Fatal("missing b")
	}
	if _, ok := reg.Get("c"); ok {
		t.Fatal("unexpected c")
	}
	if len(reg.IDs()) != 2 {
		t.Fatalf("ids: %v", reg.IDs())
	}
}
