// Package migrations embeds the forward-only SQL migrations.
//
// Files must be named <version>_<name>.sql (for example
// 0002_foundation.sql). They are applied in ascending version order and are
// never edited once applied: a change to history is a new migration.
package migrations

import "embed"

// FS contains every SQL migration file.
//
//go:embed *.sql
var FS embed.FS
