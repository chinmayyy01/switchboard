package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chinmay/switchboard/internal/telephony"
	"github.com/chinmay/switchboard/internal/tenant"
)

// sip-setup writes reviewed LiveKit SIP plans for each tenant. It never calls
// the telephony API or purchases numbers; an operator applies the files with
// the LiveKit CLI after review, so provisioning stays explicit.
func main() {
	tenantDir := flag.String("tenants", "tenants", "tenant config directory")
	outDir := flag.String("out", "deploy/livekit", "output directory for LiveKit CLI plans")
	only := flag.String("tenant", "", "only plan one tenant")
	flag.Parse()

	registry, err := tenant.Load(*tenantDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	provider, err := telephony.FromEnv()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	wrote := 0
	for _, id := range registry.IDs() {
		if *only != "" && id != *only {
			continue
		}
		cfg, ok := registry.Get(id)
		if !ok {
			continue
		}
		plan, err := provider.Plan(cfg)
		if err != nil {
			if errors.Is(err, telephony.ErrNotConfigured) {
				fmt.Fprintf(os.Stderr, "skip %s: %v\n", id, err)
				continue
			}
			fmt.Fprintf(os.Stderr, "plan %s: %v\n", id, err)
			os.Exit(1)
		}
		files := map[string]any{
			id + "-inbound.json":  map[string]any{"trunk": plan.InboundTrunk},
			id + "-dispatch.json": map[string]any{"dispatch_rule": plan.DispatchRule},
		}
		if plan.OutboundTrunk != nil {
			files[id+"-outbound.json"] = map[string]any{"trunk": plan.OutboundTrunk}
		}
		for name, body := range files {
			encoded, err := json.MarshalIndent(body, "", "  ")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			path := filepath.Join(*outDir, name)
			if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			fmt.Printf("wrote %s\n", path)
		}
		fmt.Printf("  tenant %s  provider=%s  endpoint=%s  number=%s\n",
			id, plan.Provider, plan.SIPEndpoint, plan.InboundTrunk.Numbers[0])
		wrote++
	}

	if wrote == 0 {
		fmt.Fprintln(os.Stderr, "no tenant plans written; add telephony.number to a tenant config")
		os.Exit(2)
	}
	fmt.Println("\nApply with the LiveKit CLI after review (this command purchases nothing):")
	fmt.Println("  lk sip inbound create  <tenant>-inbound.json")
	fmt.Println("  lk sip dispatch create <tenant>-dispatch.json")
	fmt.Println("  lk sip outbound create <tenant>-outbound.json   # when warm transfer is enabled")
}
