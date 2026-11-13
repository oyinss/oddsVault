// Command dedupe-live reports or merges live events that describe the same
// fixture. The sports-sync worker runs the same pass every ten minutes.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/oyins/oddsvault/backend/internal/config"
	"github.com/oyins/oddsvault/backend/internal/database"
	"github.com/oyins/oddsvault/backend/internal/dedupe"
)

func main() {
	apply := flag.Bool("apply", false, "merge the duplicates; without it the command only reports")
	flag.Parse()

	cfg, err := config.Load("oddsvault-dedupe-live")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()

	result, err := dedupe.Run(ctx, pool, *apply)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("found %d, merged %d, skipped %d\n", result.Found, result.Merged, result.Skipped)
}
