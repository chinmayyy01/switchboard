package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/chinmay/switchboard/internal/auth"
)

func main() {
	email := flag.String("email", "", "account email")
	password := flag.String("password", "", "account password")
	tenant := flag.String("tenant", "rivera", "tenant id")
	owner := flag.Bool("owner", false, "create or update the owner account")
	role := flag.String("role", "viewer", "role for a standard account: manager or viewer")
	name := flag.String("name", "", "display name for a standard account")
	prune := flag.Bool("prune", false, "delete every non-owner account and session")
	flag.Parse()

	if strings.TrimSpace(*email) == "" || *password == "" {
		log.Fatal("email and password are required")
	}
	if *prune && !*owner {
		log.Fatal("prune is only supported with owner")
	}

	store, err := auth.OpenDefault()
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	var account auth.Account
	if *owner {
		account, err = store.UpsertOwner(ctx, *email, *password, *tenant)
	} else {
		account, err = store.CreateAccountWithRole(ctx, *name, *email, *password, *tenant, *role, true)
	}
	if err != nil {
		log.Fatal(err)
	}
	if *prune {
		if err := store.PruneNonOwners(ctx); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("email=%s id=%s owner=%v free_call_limit=%d\n", account.Email, account.ID, account.IsOwner, account.FreeCallLimit)
}
