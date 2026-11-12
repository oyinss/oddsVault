package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// advisoryLockKey serialises concurrent migration runs. "ODDV".
const advisoryLockKey int64 = 0x4F444456

const noTransactionMarker = "-- oddsvault:no-transaction"

// Migration is one parsed SQL migration file.
type Migration struct {
	Version  int64
	Name     string
	Filename string
	SQL      string
	Checksum string
	NoTx     bool
}

// AppliedMigration is one row of schema_migrations.
type AppliedMigration struct {
	Version   int64
	Name      string
	Checksum  string
	AppliedAt time.Time
}

// LoadMigrations parses every *.sql file in fsys and returns them in version
// order. Validation is strict: duplicate versions, missing prefixes and empty
// files are errors.
func LoadMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	migs := make([]Migration, 0, len(entries))
	seen := make(map[int64]string, len(entries))

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		body, err := fs.ReadFile(fsys, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		base := strings.TrimSuffix(entry.Name(), ".sql")
		prefix, name, ok := strings.Cut(base, "_")
		if !ok {
			return nil, fmt.Errorf("%s: expected <version>_<name>.sql", entry.Name())
		}
		version, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil || version <= 0 {
			return nil, fmt.Errorf("%s: version prefix must be a positive integer", entry.Name())
		}
		if name == "" {
			return nil, fmt.Errorf("%s: name after version prefix is empty", entry.Name())
		}
		if other, dup := seen[version]; dup {
			return nil, fmt.Errorf("%s: version %d is already used by %s", entry.Name(), version, other)
		}
		seen[version] = entry.Name()

		sql := strings.TrimSpace(string(body))
		if sql == "" {
			return nil, fmt.Errorf("%s: migration is empty", entry.Name())
		}
		sum := sha256.Sum256(body)
		migs = append(migs, Migration{
			Version:  version,
			Name:     name,
			Filename: entry.Name(),
			SQL:      sql,
			Checksum: hex.EncodeToString(sum[:]),
			NoTx:     strings.Contains(sql, noTransactionMarker),
		})
	}

	sort.Slice(migs, func(i, j int) bool { return migs[i].Version < migs[j].Version })
	return migs, nil
}

// Applied lists migrations recorded in the database.
func Applied(ctx context.Context, pool *pgxpool.Pool) ([]AppliedMigration, error) {
	if err := ensureMigrationTable(ctx, pool); err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx,
		`SELECT version, name, checksum, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	var out []AppliedMigration
	for rows.Next() {
		var m AppliedMigration
		if err := rows.Scan(&m.Version, &m.Name, &m.Checksum, &m.AppliedAt); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Apply runs every pending migration inside its own transaction, under an
// advisory lock, and verifies that already-applied files have not changed.
func Apply(ctx context.Context, pool *pgxpool.Pool, migs []Migration) ([]Migration, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, advisoryLockKey); err != nil {
		return nil, fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		// Best effort: the lock is released with the session regardless.
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.Exec(releaseCtx, `SELECT pg_advisory_unlock($1)`, advisoryLockKey)
	}()

	if err := ensureMigrationTable(ctx, conn); err != nil {
		return nil, err
	}

	appliedRows, err := conn.Query(ctx, `SELECT version, checksum FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	applied := make(map[int64]string)
	for appliedRows.Next() {
		var version int64
		var checksum string
		if err := appliedRows.Scan(&version, &checksum); err != nil {
			appliedRows.Close()
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[version] = checksum
	}
	appliedRows.Close()
	if err := appliedRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema_migrations: %w", err)
	}

	var newlyApplied []Migration
	for _, m := range migs {
		if stored, ok := applied[m.Version]; ok {
			if stored != m.Checksum {
				return nil, fmt.Errorf(
					"migration %s was modified after it was applied (checksum mismatch); "+
						"revert the edit or add a new migration instead", m.Filename)
			}
			continue
		}
		if err := applyOne(ctx, conn, m); err != nil {
			return newlyApplied, err
		}
		newlyApplied = append(newlyApplied, m)
	}
	return newlyApplied, nil
}

func applyOne(ctx context.Context, conn *pgxpool.Conn, m Migration) (err error) {
	start := time.Now()

	if m.NoTx {
		if _, err := conn.Exec(ctx, m.SQL); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.Filename, err)
		}
	} else {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", m.Filename, err)
		}
		defer func() {
			if err != nil {
				_ = tx.Rollback(context.WithoutCancel(ctx))
			}
		}()

		if _, err = tx.Exec(ctx, m.SQL); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.Filename, err)
		}
		if _, err = tx.Exec(ctx,
			`INSERT INTO schema_migrations (version, name, checksum, duration_ms)
			 VALUES ($1, $2, $3, $4)`,
			m.Version, m.Name, m.Checksum, time.Since(start).Milliseconds()); err != nil {
			return fmt.Errorf("record migration %s: %w", m.Filename, err)
		}
		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.Filename, err)
		}
		return nil
	}

	if _, err := conn.Exec(ctx,
		`INSERT INTO schema_migrations (version, name, checksum, duration_ms)
		 VALUES ($1, $2, $3, $4)`,
		m.Version, m.Name, m.Checksum, time.Since(start).Milliseconds()); err != nil {
		return fmt.Errorf("record migration %s: %w", m.Filename, err)
	}
	return nil
}

type execQuerier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func ensureMigrationTable(ctx context.Context, q execQuerier) error {
	_, err := q.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     bigint      PRIMARY KEY,
			name        text        NOT NULL,
			checksum    text        NOT NULL,
			applied_at  timestamptz NOT NULL DEFAULT now(),
			duration_ms integer     NOT NULL
		)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}
