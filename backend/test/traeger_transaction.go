package test

import "maps"

// traegerTransactionSites are the boundaries of the Träger transaction ADR
// 0086 section 5 decides and ADR 0105 introduces: the tenant runtime's entry
// point, its PostgreSQL binding and adapter, and the one HTTP middleware that
// opens it for a Träger-Portal route. No owner command opens it itself; a
// further site needs its own decision.
var traegerTransactionSites = map[string]string{
	"tenant/traeger.go:WithinTraeger:runtime.withinTraeger#1":                                                               "ADR 0086 section 5 and ADR 0105: the tenant runtime's Träger transaction entry point refuses a school, administrative or foreign Träger unit and binds the Träger for the adapter; never selected by a missing school.",
	"modules/transactionruntime/compose/tenant_runtime.go:NewTenantRuntime:unitOfWork.WithinTraeger#1":                      "ADR 0086 section 5 and ADR 0105: Transaction Runtime binds the PostgreSQL Träger transaction into the serving unit of work; the demo runtime gets none.",
	"modules/transactionruntime/internal/adapters/postgres/tenant_runtime.go:*PostgresUnitOfWork.WithinTraeger:r.runInTx#1": "ADR 0086 section 5 and ADR 0105: the Träger transaction runs under phoenix_tenant with app.current_org_id and no school, joins only the same Träger's ambient unit, and never assumes the administrative role.",
	"api/common/tenant_middleware.go:TraegerTxMiddleware:tenant.WithinTraeger#1":                                            "ADR 0105: TraegerTxMiddleware opens the Träger transaction of the session's Träger for a Träger-Portal route; a request without a Träger session fails closed before the handler.",
}

// withoutReviewedTraegerTransactions removes exactly the reviewed Träger
// transaction boundaries, with their reviewed reasons, before the
// shrink-only comparison. It is no general escape: any other new or reworded
// site stays.
func withoutReviewedTraegerTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range traegerTransactionSites {
		if current[site] != reason {
			continue
		}
		if len(remaining) == len(current) {
			remaining = maps.Clone(current)
		}
		delete(remaining, site)
	}
	return remaining
}
