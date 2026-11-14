package tenant

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidName     = errors.New("business name is required")
	ErrInvalidServices = errors.New("at least one service is required")
	ErrInvalidWindows  = errors.New("choose at least one valid window")
)

// Validate normalizes and checks the editable tenant fields. The tenant ID is
// immutable and is not changed here.
func Validate(t Tenant) (Tenant, error) {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" || len([]rune(t.Name)) > 120 {
		return Tenant{}, ErrInvalidName
	}
	t.Services = cleanList(t.Services, 20, 60)
	if len(t.Services) == 0 {
		return Tenant{}, ErrInvalidServices
	}
	allowedWindows := map[string]bool{"morning": true, "afternoon": true, "evening": true}
	var windows []string
	seen := map[string]bool{}
	for _, window := range t.Windows {
		window = strings.ToLower(strings.TrimSpace(window))
		if allowedWindows[window] && !seen[window] {
			seen[window] = true
			windows = append(windows, window)
		}
	}
	if len(windows) == 0 {
		return Tenant{}, ErrInvalidWindows
	}
	t.Windows = windows
	t.Persona.AgentName = strings.TrimSpace(t.Persona.AgentName)
	if t.Persona.AgentName == "" {
		t.Persona.AgentName = "Alex"
	}
	t.Persona.Tone = strings.TrimSpace(t.Persona.Tone)
	if t.Persona.Tone == "" {
		t.Persona.Tone = "warm, brief, and natural"
	}
	t.Persona.Languages = cleanList(t.Persona.Languages, 8, 16)
	if len(t.Persona.Languages) == 0 {
		t.Persona.Languages = []string{"en", "hi"}
	}
	t.ManagerNumber = strings.TrimSpace(t.ManagerNumber)
	t.Disclosure = strings.TrimSpace(t.Disclosure)
	if len([]rune(t.Disclosure)) > 2000 {
		return Tenant{}, fmt.Errorf("disclosure must be under 2000 characters")
	}
	return t, nil
}

func cleanList(values []string, maxItems, maxLen int) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len([]rune(value)) > maxLen {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

// Save writes config.json atomically. The tenant directory holds config and
// staff; only config.json is changed here.
func Save(dir string, t Tenant) (Tenant, error) {
	validated, err := Validate(t)
	if err != nil {
		return Tenant{}, err
	}
	root := filepath.Join(dir, validated.ID)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Tenant{}, err
	}
	encoded, err := json.MarshalIndent(validated, "", "  ")
	if err != nil {
		return Tenant{}, err
	}
	temp, err := os.CreateTemp(root, "config-*.tmp")
	if err != nil {
		return Tenant{}, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(append(encoded, '\n')); err != nil {
		temp.Close()
		return Tenant{}, err
	}
	if err := temp.Close(); err != nil {
		return Tenant{}, err
	}
	if err := os.Rename(tempName, filepath.Join(root, "config.json")); err != nil {
		return Tenant{}, err
	}
	return validated, nil
}
