package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/tenantexport"
	"github.com/uptrace/bun"
)

// NewTenantExporter binds only the administrative return procedure. Unlike
// HTTP services it owns its read-only snapshot, not an ambient request scope.
func NewTenantExporter(db *bun.DB, fingerprints organizationtenancy.ExportFingerprints) (organizationtenancy.TenantExporter, error) {
	if db == nil || fingerprints.Bytes == nil || fingerprints.Reader == nil {
		return nil, errors.New("tenant export requires a database")
	}
	return tenantexport.NewTenantExporter(db, fingerprints), nil
}
