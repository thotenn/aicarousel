package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/thotenn/aicarousel-go/internal/cli"
	"github.com/thotenn/aicarousel-go/internal/config"
	"github.com/thotenn/aicarousel-go/internal/db"
	"github.com/thotenn/aicarousel-go/internal/db/apikeys"
	"github.com/thotenn/aicarousel-go/internal/db/provsettings"
)

var version = "dev"

func main() {
	// Load .env before anything else so DB_PATH etc. are available.
	config.Load("")

	// Open database (migrations will be run on demand inside the CLI).
	d, err := db.Open(config.Cfg.DBPath)
	if err != nil {
		// DB might not exist yet — that's OK. setup option 1 will create it.
		slog.Warn("could not open database (run Setup to initialize)", "err", err)
		// Still launch the menu so the user can run setup.
		d = nil
	}

	// If we couldn't open the DB (data dir missing), create a temporary
	// in-memory placeholder so repos don't nil-pointer. The setup flow will
	// reinitialize with the correct path.
	if d == nil {
		if err := os.MkdirAll("data", 0o755); err == nil {
			d, err = db.Open(config.Cfg.DBPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "aicarousel-setup: fatal: cannot open DB: %v\n", err)
				os.Exit(1)
			}
		}
	}
	if d != nil {
		defer d.Close() //nolint:errcheck
		// Apply any pending migrations silently.
		_ = db.Up(d)
	}

	app := &cli.App{
		DB:   d,
		Keys: apikeys.New(d),
		Prov: provsettings.New(d),
	}

	fmt.Printf("AICarousel Setup v%s\n", version)
	cli.Run(app)
}
