// Command analytics-worker rebuilds derived analytics. Materialisations are
// registered in later phases, once there is settled history to aggregate.
package main

import (
	"fmt"
	"os"

	"github.com/oyins/oddsvault/backend/internal/worker"
)

func main() {
	// Analytics jobs are registered in Phase 6.
	if err := worker.Boot("oddsvault-analytics-worker", nil); err != nil {
		fmt.Fprintf(os.Stderr, "analytics-worker: %v\n", err)
		os.Exit(1)
	}
}
