package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/thotenn/aicarousel-go/internal/config"
	"github.com/thotenn/aicarousel-go/internal/db"
	"github.com/thotenn/aicarousel-go/internal/db/apikeys"
)

// version is set at build time via -ldflags "-X main.version=<ver>".
var version = "dev" //nolint:unused

const usageTmpl = `API Key Management — AICarousel

Usage:
  aicarousel-apikey create [name]   Create a new API key
  aicarousel-apikey list            List all API keys
  aicarousel-apikey revoke <id>     Revoke an API key (disable)
  aicarousel-apikey delete <id>     Delete an API key permanently

Examples:
  aicarousel-apikey create "Production"
  aicarousel-apikey create "Development"
  aicarousel-apikey list
  aicarousel-apikey revoke 1
  aicarousel-apikey delete 2

Exit codes: 0 success · 1 usage error · 2 runtime error
`

// exit codes
const (
	exitOK      = 0
	exitUsage   = 1
	exitRuntime = 2
)

func main() {
	config.Load("")

	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Print(usageTmpl)
		os.Exit(exitUsage)
	}

	command := args[0]
	rest := args[1:]

	d, err := db.Open(config.Cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot open database: %v\n", err)
		fmt.Fprintf(os.Stderr, "Run 'aicarousel-setup' first to initialize the database.\n")
		os.Exit(exitRuntime)
	}
	defer d.Close() //nolint:errcheck

	if err := db.Up(d); err != nil {
		fmt.Fprintf(os.Stderr, "error: migrations failed: %v\n", err)
		os.Exit(exitRuntime)
	}

	repo := apikeys.New(d)
	ctx := context.Background()

	switch command {
	case "create":
		name := ""
		if len(rest) > 0 {
			name = strings.Join(rest, " ")
		}
		cmdCreate(ctx, repo, name)

	case "list":
		cmdList(ctx, repo)

	case "revoke":
		if len(rest) == 0 {
			fmt.Fprintln(os.Stderr, "error: 'revoke' requires an <id> argument")
			os.Exit(exitUsage)
		}
		id := parseID(rest[0])
		cmdRevoke(ctx, repo, id)

	case "delete":
		if len(rest) == 0 {
			fmt.Fprintln(os.Stderr, "error: 'delete' requires an <id> argument")
			os.Exit(exitUsage)
		}
		id := parseID(rest[0])
		cmdDelete(ctx, repo, id)

	default:
		fmt.Fprintf(os.Stderr, "error: unknown command %q\n\n", command)
		fmt.Print(usageTmpl)
		os.Exit(exitUsage)
	}
}

// parseID parses a string as a positive int64 ID, printing an error and
// exiting with exitUsage if invalid.
func parseID(s string) int64 {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 {
		fmt.Fprintf(os.Stderr, "error: invalid id %q — must be a positive integer\n", s)
		os.Exit(exitUsage)
	}
	return id
}

// cmdCreate creates a new API key and prints it to stdout.
func cmdCreate(ctx context.Context, repo apikeys.Repo, name string) {
	plain, rec, err := repo.Create(ctx, name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(exitRuntime)
	}

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    🔑 NEW API KEY CREATED                          ║")
	fmt.Println("╠════════════════════════════════════════════════════════════════════╣")
	fmt.Println("║                                                                    ║")
	fmt.Printf("║  %-68s║\n", plain)
	fmt.Println("║                                                                    ║")
	fmt.Println("╠════════════════════════════════════════════════════════════════════╣")
	fmt.Println("║  ⚠️  Save this key now. You won't be able to see it again!         ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Details:")
	fmt.Printf("  ID:      %d\n", rec.ID)
	n := "(none)"
	if rec.Name.Valid {
		n = rec.Name.String
	}
	fmt.Printf("  Name:    %s\n", n)
	fmt.Printf("  Prefix:  %s\n", rec.KeyPrefix)
	fmt.Printf("  Created: %s\n", rec.CreatedAt)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  curl -H \"Authorization: Bearer %s\" http://localhost:7123/v1/chat/completions ...\n", plain)
	fmt.Printf("  curl -H \"x-api-key: %s\" http://localhost:7123/v1/messages ...\n", plain)
	fmt.Println()
}

// cmdList prints all API keys in an ASCII table.
func cmdList(ctx context.Context, repo apikeys.Repo) {
	keys, err := repo.List(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(exitRuntime)
	}

	if len(keys) == 0 {
		fmt.Println()
		fmt.Println("No API keys found. Create one with: aicarousel-apikey create [name]")
		fmt.Println()
		return
	}

	fmt.Println()
	fmt.Println("┌────┬─────────────┬──────────────────┬─────────────────────┬─────────────────────┬────────┬───────────┐")
	fmt.Println("│ ID │ Prefix      │ Name             │ Created             │ Last Used           │ Active │ Usage     │")
	fmt.Println("├────┼─────────────┼──────────────────┼─────────────────────┼─────────────────────┼────────┼───────────┤")

	for _, k := range keys {
		id := padRight(strconv.FormatInt(k.ID, 10), 2)
		prefix := padRight(k.KeyPrefix, 11)
		name := padRight(truncate(nullStr(k.Name.String, k.Name.Valid, "-"), 16), 16)
		created := padRight(truncate(k.CreatedAt, 19), 19)
		lastUsed := padRight(truncate(nullStr(k.LastUsedAt.String, k.LastUsedAt.Valid, "-"), 19), 19)
		active := padRight(boolStr(k.IsActive == 1), 6)
		usageStr := padRight(strconv.FormatInt(k.UsageCount, 10), 9)

		fmt.Printf("│ %s │ %s │ %s │ %s │ %s │ %s │ %s │\n",
			id, prefix, name, created, lastUsed, active, usageStr)
	}

	fmt.Println("└────┴─────────────┴──────────────────┴─────────────────────┴─────────────────────┴────────┴───────────┘")
	fmt.Println()
}

// cmdRevoke disables an API key.
func cmdRevoke(ctx context.Context, repo apikeys.Repo, id int64) {
	found, err := repo.Revoke(ctx, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(exitRuntime)
	}
	if !found {
		fmt.Fprintf(os.Stderr, "\n✗ API key %d not found\n\n", id)
		os.Exit(exitRuntime)
	}
	fmt.Printf("\n✓ API key %d has been revoked\n\n", id)
}

// cmdDelete permanently removes an API key.
func cmdDelete(ctx context.Context, repo apikeys.Repo, id int64) {
	found, err := repo.Delete(ctx, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(exitRuntime)
	}
	if !found {
		fmt.Fprintf(os.Stderr, "\n✗ API key %d not found\n\n", id)
		os.Exit(exitRuntime)
	}
	fmt.Printf("\n✓ API key %d has been deleted\n\n", id)
}

// --- string helpers ---

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func nullStr(s string, valid bool, fallback string) string {
	if !valid {
		return fallback
	}
	return s
}

func boolStr(v bool) string {
	if v {
		return "✓"
	}
	return "✗"
}
