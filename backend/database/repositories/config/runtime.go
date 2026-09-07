package config

import (
	"context"

	"github.com/uptrace/bun"
)

// Runtime supplies the ambient transaction without coupling the settings
// Postgres adapter to the transaction implementation. Tenant scoping is left
// to the transaction's row-level security policy.
type Runtime interface {
	DB(context.Context) bun.IDB
}

type directRuntime struct{ db *bun.DB }

// NewRuntime provides the non-transactional runtime used by composition
// callers that only perform privileged reads. Application services replace it
// with their tenant-aware runtime before wiring settings repositories.
func NewRuntime(db *bun.DB) Runtime { return directRuntime{db: db} }

func (r directRuntime) DB(context.Context) bun.IDB { return r.db }
