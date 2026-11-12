package test

import "maps"

const visitRetentionTransactionSite = "modules/studentpresence/internal/application/presence/cleanup_service.go:*cleanupService.expireVisitsInTx:s.txHandler.RunInTx#1"
const visitRetentionTransactionReason = "Batch and per-child recovery unit (#6455, ADR 0199): expireVisitsInTx deletes the expired visits of a batch and appends their audit atomically. A failed batch rolls back whole and reruns one child per transaction, so the cleanup records an individual failure and continues; no commit preserves a deletion without its audit."

// ADR 0199 (#6455) moves the visit retention's recovery unit from one child
// (processStudent) to one batch with a per-child rerun (expireVisitsInTx).
// Only that exact site and reason are admitted; any other new or reworded
// site stays rejected.
func withoutReviewedVisitRetentionTransaction(current map[string]string) map[string]string {
	if current[visitRetentionTransactionSite] != visitRetentionTransactionReason {
		return current
	}
	remaining := maps.Clone(current)
	delete(remaining, visitRetentionTransactionSite)
	return remaining
}
