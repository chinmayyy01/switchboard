package tenant

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Provision creates a new tenant directory with a usable default config and an
// empty staff list. The caller registers it with the running Registry.
func Provision(dir, name string, exists func(string) bool) (Tenant, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Tenant{}, fmt.Errorf("business name is required")
	}
	id := ""
	for attempt := 0; attempt < 8; attempt++ {
		candidate := slug(name)
		if candidate == "" {
			candidate = "business"
		}
		suffix, err := randomSuffix()
		if err != nil {
			return Tenant{}, err
		}
		id = candidate + "-" + suffix
		if exists == nil || !exists(id) {
			break
		}
		id = ""
	}
	if id == "" {
		return Tenant{}, fmt.Errorf("could not allocate a tenant id")
	}

	cfg := Tenant{
		ID:       id,
		Name:     name,
		Services: []string{"repair", "installation", "maintenance", "emergency repair"},
		Windows:  []string{"morning", "afternoon", "evening"},
		Persona: Persona{
			AgentName: "Alex",
			Tone:      "warm, brief, and natural",
			Languages: []string{"en", "hi"},
		},
		Disclosure: "Hi! This is an AI voice assistant for " + name +
			". I can take your details and book a visit. " +
			"I can keep a text transcript of this call. Are you comfortable with that, or would you prefer I don't save one?",
	}

	root := filepath.Join(dir, id)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Tenant{}, err
	}
	saved, err := Save(dir, cfg)
	if err != nil {
		return Tenant{}, err
	}
	cfg = saved
	if err := os.WriteFile(filepath.Join(root, "staff.json"), []byte("{\n  \"employees\": []\n}\n"), 0o644); err != nil {
		return Tenant{}, err
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		return Tenant{}, err
	}
	return cfg, nil
}

func slug(value string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(value) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.':
			if !lastHyphen && b.Len() > 0 {
				b.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len([]rune(out)) > 32 {
		out = string([]rune(out)[:32])
		out = strings.Trim(out, "-")
	}
	return out
}

func randomSuffix() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
