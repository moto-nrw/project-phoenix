package test

import "maps"

const tenantExportTransactionSite = "modules/organizationtenancy/internal/adapters/tenantexport/tenant_export.go:*tenantExporter.start:e.db.BeginTx#1"
const tenantExportTransactionReason = "ADR 0092 and ADR 0097: internal school return owns one REPEATABLE READ READ ONLY snapshot under the NOLOGIN export reader; explicit catalog school predicates and secret projections remain mandatory."

// ADR 0097 admits one new administrative read boundary, not a general escape
// from the unchanged relocation/shrink-only contract for existing commands.
func withoutReviewedTenantExportTransaction(current map[string]string) map[string]string {
	if current[tenantExportTransactionSite] != tenantExportTransactionReason {
		return current
	}
	remaining := maps.Clone(current)
	delete(remaining, tenantExportTransactionSite)
	return remaining
}

const returnRegisterTransactionSite = "modules/organizationtenancy/internal/adapters/tenantexport/tenant_export_register.go:*returnRegisterBuilder.BuildReturnRegister:b.db.BeginTx#1"
const returnRegisterTransactionReason = "ADR 0201: the register of the return package reads only the schema, in one READ ONLY transaction under the NOLOGIN export reader, the way the export reads it; it reads no row."

// ADR 0201 admits the schema read of the return register beside the export
// snapshot, and nothing else.
func withoutReviewedReturnRegisterTransaction(current map[string]string) map[string]string {
	if current[returnRegisterTransactionSite] != returnRegisterTransactionReason {
		return current
	}
	remaining := maps.Clone(current)
	delete(remaining, returnRegisterTransactionSite)
	return remaining
}
