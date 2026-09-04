package activities

import (
	"context"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// WrapDatabaseError preserves the legacy repository error contract for
// composition-layer adapters that cannot import legacy model infrastructure.
func WrapDatabaseError(operation string, err error) error {
	return &modelBase.DatabaseError{Op: operation, Err: err}
}

// WrapNotFoundDatabaseError preserves the typed legacy not-found contract.
func WrapNotFoundDatabaseError(operation string) error {
	return WrapDatabaseError(operation, modelBase.ErrNotFound)
}

// ContextTenantMatches guards compatibility methods whose old signature
// redundantly carries the tenant beside the authenticated context.
func ContextTenantMatches(ctx context.Context, tenantID int64) bool {
	contextTenant, err := tenant.TenantFromContext(ctx)
	return err == nil && contextTenant.Int64() == tenantID
}
