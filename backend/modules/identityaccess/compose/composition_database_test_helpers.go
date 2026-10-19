//go:build testhooks

package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func newStore(db *bun.DB) *postgres.Store             { return postgres.NewStore(db) }
func newTransactionRunner() *tenant.TransactionRunner { return tenant.NewTransactionRunner() }
