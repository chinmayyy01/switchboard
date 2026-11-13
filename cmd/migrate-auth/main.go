package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/chinmay/switchboard/internal/auth"
)

func main() {
	sqlitePath := flag.String("sqlite", envOr("AUTH_DB", "data/auth.db"), "source SQLite database")
	postgresDSN := flag.String("postgres", os.Getenv("DATABASE_URL"), "target Postgres DSN")
	dryRun := flag.Bool("dry-run", false, "list accounts without writing")
	flag.Parse()

	if *postgresDSN == "" {
		log.Fatal("DATABASE_URL or -postgres is required")
	}
	src, err := auth.Open(*sqlitePath)
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()
	accounts, err := src.ExportAccounts(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	if *dryRun {
		for _, a := range accounts {
			fmt.Printf("would import email=%s owner=%v tenant=%s\n", a.Email, a.IsOwner, a.TenantID)
		}
		return
	}
	dst, err := auth.OpenPostgres(*postgresDSN)
	if err != nil {
		log.Fatal(err)
	}
	defer dst.Close()
	for _, a := range accounts {
		if err := dst.ImportAccount(context.Background(), a); err != nil {
			log.Fatalf("import %s: %v", a.Email, err)
		}
	}
	fmt.Printf("imported %d accounts; sessions were not migrated, sign in again\n", len(accounts))
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
