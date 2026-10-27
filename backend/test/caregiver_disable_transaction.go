package test

import "maps"

const caregiverDisableTransactionSite = "modules/peopledirectory/internal/application/caregiver_capability.go:*caregiverCapabilityService.DisableCaregiverCapability:tenant.WithinTenantRetry#1"
const caregiverDisableTransactionReason = "Independent owner command: DisableCaregiverCapability masks the caller's transaction and after-commit hooks and opens its own retry-safe tenant unit, because it LOCK TABLEs the caregiver-binding tables and a deadlock replay needs a transaction it owns. Its prerequisite reads/locks, mutations, audit and commit hooks roll back together in that unit; it commits before the caller does, so a later write of the caller is not atomic with it."

// #4605 rewords the register entry of an unchanged boundary to match its code:
// the command opens its own unit instead of joining the caller's. Only that
// exact reason is admitted; any other new or reworded site stays rejected.
func withoutReviewedCaregiverDisableTransaction(current map[string]string) map[string]string {
	if current[caregiverDisableTransactionSite] != caregiverDisableTransactionReason {
		return current
	}
	remaining := maps.Clone(current)
	delete(remaining, caregiverDisableTransactionSite)
	return remaining
}
