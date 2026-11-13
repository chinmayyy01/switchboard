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
	"github.com/chinmay/switchboard/internal/storage"
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
	recordingDays := flag.Int("recordings-days", envDays("RECORDING_RETENTION_DAYS", 30), "audio retention in days")
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
	objects, err := storage.NewFromEnv()
	if err != nil {
		slog.Error("storage", "err", err)
		os.Exit(1)
	}

	recordingCutoff := time.Now().AddDate(0, 0, -*recordingDays)
	expired, err := store.ExpiredRecordings(ctx, recordingCutoff)
	if err != nil {
		slog.Error("list expired recordings", "err", err)
		os.Exit(1)
	}
	deleted := 0
	for _, rec := range expired {
		if *dryRun {
			slog.Info("would delete recording", "call_id", rec.CallID, "key", rec.ObjectKey)
			continue
		}
		if objects == nil || !objects.Configured() {
			slog.Error("recording storage is not configured; refusing to mark rows deleted", "call_id", rec.CallID)
			continue
		}
		if err := objects.Delete(ctx, rec.ObjectKey); err != nil {
			slog.Error("delete object", "call_id", rec.CallID, "key", rec.ObjectKey, "err", err)
			continue
		}
		if err := store.MarkRecordingDeleted(ctx, rec.CallID); err != nil {
			slog.Error("mark recording deleted", "call_id", rec.CallID, "err", err)
			continue
		}
		deleted++
	}

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
		"recordings_expired", len(expired),
		"recordings_deleted", deleted,
		"transcript_turns_deleted", turns,
		"dry_run", *dryRun)
}
