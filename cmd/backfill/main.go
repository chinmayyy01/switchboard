package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/chinmay/switchboard/internal/jobs"
)

type turn struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type record struct {
	ID         string    `json:"id"`
	Tenant     string    `json:"tenant"`
	Lang       string    `json:"lang"`
	Escalated  bool      `json:"escalated"`
	Reason     string    `json:"reason"`
	Transcript []turn    `json:"transcript"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at"`
	Label      *bool     `json:"label"`
}

func main() {
	dir := flag.String("data", "data", "legacy data directory")
	transcripts := flag.Bool("transcripts", false, "import legacy transcripts (requires a documented consent basis)")
	flag.Parse()

	ctx := context.Background()
	db, err := jobs.OpenPostgres(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	labels := map[string]bool{}
	if data, err := os.ReadFile(*dir + "/labels.json"); err == nil {
		_ = json.Unmarshal(data, &labels)
	}

	file, err := os.Open(*dir + "/calls.jsonl")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	imported, skipped := 0, 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var rec record
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil || rec.ID == "" {
			skipped++
			continue
		}
		if rec.Tenant == "" {
			rec.Tenant = "rivera"
		}
		ended := rec.EndedAt
		if ended.IsZero() {
			ended = rec.StartedAt.Add(time.Minute)
		}
		if _, err := db.ExecContext(ctx, `
INSERT INTO calls (id, tenant_id, direction, room_name, started_at, ended_at, outcome,
                   escalated, escalation_reason, transcript_consent, consent_version, source, consent_language)
VALUES ($1, $2, 'browser', '', $3, $4, 'completed', $5, $6, $7, 'legacy-import', 'legacy', $8)
ON CONFLICT (id) DO NOTHING`,
			rec.ID, rec.Tenant, rec.StartedAt, ended, rec.Escalated, rec.Reason,
			*transcripts, rec.Lang); err != nil {
			skipped++
			continue
		}
		if *transcripts {
			for i, item := range rec.Transcript {
				role := "assistant"
				if item.Role == "caller" {
					role = "user"
				}
				if _, err := db.ExecContext(ctx, `
INSERT INTO call_turns (call_id, tenant_id, seq, role, text, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (call_id, seq) DO NOTHING`,
					rec.ID, rec.Tenant, i+1, role, item.Text, rec.StartedAt); err != nil {
					skipped++
				}
			}
		}
		label, ok := labels[rec.ID]
		if rec.Label != nil {
			label, ok = *rec.Label, true
		}
		if ok {
			_, _ = db.ExecContext(ctx, `
INSERT INTO call_labels (call_id, tenant_id, label) VALUES ($1, $2, $3)
ON CONFLICT (call_id) DO UPDATE SET label = EXCLUDED.label`, rec.ID, rec.Tenant, label)
		}
		imported++
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	log.Printf("imported=%d skipped=%d transcripts=%v", imported, skipped, *transcripts)
}
