package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/tenantexport"
	"github.com/uptrace/bun"
)

// NewTenantExporter binds only the administrative return procedure. Unlike
// HTTP services it owns its read-only snapshot, not an ambient request scope.
func NewTenantExporter(db *bun.DB, fingerprints organizationtenancy.ExportFingerprints, sealed organizationtenancy.TenantExportSealedFields, projected organizationtenancy.TenantExportProjectedFields) (organizationtenancy.TenantExporter, error) {
	if db == nil || fingerprints.Bytes == nil || fingerprints.Reader == nil {
		return nil, errors.New("tenant export requires a database")
	}
	return tenantexport.NewTenantExporter(db, organizationtenancy.TenantExportDependencies{
		Fingerprints: fingerprints, Sealed: sealed, Projected: projected}), nil
}

// NewTraegerTenantExporter binds the return procedure with what a Träger
// package needs beyond the catalog (ADR 0187): the PAIN.008 files Parent
// Billing opens and the operator audit of each run. A school export runs
// through it unchanged.
func NewTraegerTenantExporter(db *bun.DB, dependencies organizationtenancy.TenantExportDependencies) (organizationtenancy.TenantExporter, error) {
	if db == nil || dependencies.Fingerprints.Bytes == nil || dependencies.Fingerprints.Reader == nil ||
		dependencies.SEPADebitFiles == nil || dependencies.Audit == nil {
		return nil, errors.New("tenant export of a Träger requires a database, its PAIN.008 files and its operator audit")
	}
	return tenantexport.NewTenantExporter(db, dependencies), nil
}
