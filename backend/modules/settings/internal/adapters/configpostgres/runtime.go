package config

import (
	"context"

	"github.com/uptrace/bun"
)

// Runtime supplies the ambient transaction without coupling the settings
// Postgres adapter to the transaction implementation. Tenant scoping is left
// to the transaction's row-level security policy.
type Runtime interface {
	DB(context.Context) (bun.IDB, error)
}
