package migrations

import (
	"context"
	"log/slog"
)

// This file holds the logging a migration body is allowed to do (#3300).
//
// The runner writes one line per migration — version, description and
// duration_ms — so a migration no longer announces itself. What a migration
// still owns is output that carries content: how many rows a repair touched,
// which records it could not fix, a rollback that failed. That goes through
// slog with fields, so the value is a field Loki can filter on rather than a
// number baked into a sentence.
//
// Migrations run from the CLI and take no logger: their signature is
// func(ctx context.Context, db *bun.DB) error. The process logger is the one
// the migrate command installs through applog, so slog.Default() is the
// configured logger here, not a bare fallback.

// migrationLog returns the logger a migration body writes content with, scoped
// to the same component as the runner's own lines.
func migrationLog() *slog.Logger {
	return slog.Default().With("component", migrationLogComponent)
}

// logRollbackFailure reports a deferred transaction rollback that itself failed.
//
// Migrations open their own transactions and defer a rollback that is a no-op
// once the work committed — that case is filtered at the call site and never
// reaches here. What reaches here is a rollback that returned a real error,
// which means the transaction may still hold its locks. It is reported rather
// than returned because the deferred call runs after the migration has already
// decided what to return, and the original error is the more useful one.
func logRollbackFailure(ctx context.Context, err error) {
	migrationLog().WarnContext(ctx, "migration transaction rollback failed",
		"error", err,
	)
}
