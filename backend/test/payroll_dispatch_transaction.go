package test

import "maps"

// payrollDispatchTransactionSites are the boundaries ADR 0141 adds for the
// automatic send to the payroll office: the e-mail record of the transfer
// journal, the steps of a dispatch pass, and the public download that runs in
// the school its link names. A further site needs its own decision.
var payrollDispatchTransactionSites = map[string]string{
	"modules/exporttransfer/internal/application/service.go:*Service.RecordEmail:s.runtime.WithinTenant#1":               "ADR 0141: RecordEmail journals one settled e-mail send of an export in the caller's tenant transaction, so the journal entry commits with the dispatch outcome that names it.",
	"modules/workforce/internal/application/payroll_dispatch.go:*PayrollDispatch.Download:s.deps.Runtime.WithinTenant#1": "ADR 0141: the public payroll download spends its one-time link in the transaction of the school the link names; marking the download and clearing the file commit together.",
	"modules/workforce/internal/application/payroll_dispatch.go:NewTenantPayrollDispatch:tenant.WithinCurrentTenant#1":   "ADR 0141: each step of a payroll dispatch pass (claim, send, settle, finish, purge) commits in its own tenant transaction fenced by the claim's lease, so a crashed worker leaves a retryable row instead of a lost file.",
	"modules/workforce/internal/application/payroll_dispatch.go:NewTenantPayrollDispatch:tenant.WithinTenant#1":          "ADR 0141: the public payroll download carries no tenant; the link names the school and the spend runs in that school's transaction under row-level security.",
}

// withoutReviewedPayrollDispatchTransactions removes exactly the reviewed
// boundaries, with their reviewed reasons, before the shrink-only
// comparison. It is no general escape: any other new or reworded site stays.
func withoutReviewedPayrollDispatchTransactions(current map[string]string) map[string]string {
	remaining := current
	for site, reason := range payrollDispatchTransactionSites {
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
