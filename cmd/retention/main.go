package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/chinmay/switchboard/internal/calls"
	"github.com/chinmay/switchboard/internal/jobs"
)

func envDays(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func main() {
	transcriptDays := flag.Int("transcripts-days", envDays("TRANSCRIPT_RETENTION_DAYS", 180), "transcript retention in days")
	dryRun := flag.Bool("dry-run", false, "report what would be deleted")
	flag.Parse()

	ctx := context.Background()
	db, err := jobs.OpenPostgres(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("database", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	store := calls.New(db)

	transcriptCutoff := time.Now().AddDate(0, 0, -*transcriptDays)
	var turns int64
	if *dryRun {
		slog.Info("would delete transcripts older than", "cutoff", transcriptCutoff)
	} else {
		turns, err = store.DeleteTranscriptsBefore(ctx, transcriptCutoff)
		if err != nil {
			slog.Error("delete transcripts", "err", err)
			os.Exit(1)
		}
	}
	slog.Info("retention complete",
		"transcript_turns_deleted", turns,
		"dry_run", *dryRun)
}
