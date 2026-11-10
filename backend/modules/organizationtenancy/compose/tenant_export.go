package compose

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/datareturn"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/tenantexport"
	"github.com/uptrace/bun"
)

// NewTenantExporter binds only the administrative return procedure, audited
// per run (#6326); it owns its read-only snapshot, not a request scope.
func NewTenantExporter(db *bun.DB, fingerprints organizationtenancy.ExportFingerprints, sealed organizationtenancy.TenantExportSealedFields, projected organizationtenancy.TenantExportProjectedFields, audit organizationtenancy.TenantExportAudit) (organizationtenancy.TenantExporter, error) {
	if db == nil || fingerprints.Bytes == nil || fingerprints.Reader == nil || audit == nil {
		return nil, errors.New("tenant export requires a database and its operator audit")
	}
	return tenantexport.NewTenantExporter(db, organizationtenancy.TenantExportDependencies{
		Fingerprints: fingerprints, Sealed: sealed, Projected: projected, Audit: audit}), nil
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

// NewDataReturn binds the HTTPS retrieval of released packages (ADR 0203).
func NewDataReturn(db *bun.DB, crypto organizationtenancy.DataReturnCrypto, root string, logger *slog.Logger) (organizationtenancy.DataReturn, error) {
	if db == nil || !crypto.Complete() || root == "" {
		return nil, errors.New("data return requires a database and its cryptography")
	}
	return datareturn.New(db, crypto, nil, root, logger), nil
}

// NewDataReturnRelease binds the operator side of the retrieval, audited.
func NewDataReturnRelease(db *bun.DB, crypto organizationtenancy.DataReturnCrypto, audit organizationtenancy.DataReturnAudit, root string) (organizationtenancy.DataReturnRelease, error) {
	if db == nil || !crypto.Complete() || audit == nil || root == "" {
		return nil, errors.New("data return release requires a database, its cryptography and the operator audit")
	}
	return datareturn.New(db, crypto, audit, root, nil), nil
}
