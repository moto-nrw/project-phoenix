package compose

import (
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/auditlog"
	"github.com/moto-ogs/project-phoenix/modules/auditlog/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

func NewCorrectionQueries(db *bun.DB, observe func(auditlog.Observation)) (auditlog.CorrectionQuery, error) {
	if db == nil || observe == nil {
		return nil, errors.New("audit correction queries: database and observer are required")
	}
	store := postgres.NewCorrections(postgres.NewCorrectionRuntime(db, tenant.TenantFromContext, tenant.TransactionFromContext), observe)
	return store, nil
}
