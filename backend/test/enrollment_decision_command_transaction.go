package test

import "maps"

const enrollmentDecisionCommandTransactionSite = "modules/enrollment/internal/application/decide_and_invite.go:*Decisions.decideAndInviteOnce:d.deps.Runtime.Transactions.RunInTx#1"
const enrollmentDecisionCommandTransactionReason = "Atomic owner command: DecideAndInvite applies one review decision and creates the guardian invitation it owes through application ports (#6093). The decision, its mail outbox and the Identity & Access invitation join one unit and must roll back together. It joins a supplied root unit; standalone it opens one and replays it once after a transient failure before the decision body ran. SQL adapters below it require that unit and cannot open one."

// enrollmentDecisionHandlerTransactionSite is the HTTP boundary #6093 moved
// into the owner command above.
const enrollmentDecisionHandlerTransactionSite = "modules/enrollment/http/admin_decision_handlers.go:*Resource.decideChildWithRetry:rs.runInTenantTx#1"

// #6093 moves the decision unit, its retry and the guardian invitation from
// the HTTP handler into Enrollment's DecideAndInvite command. The register
// trades the handler boundary for the owner boundary: only that exact reason
// is admitted, and only while the handler boundary is gone.
func withoutReviewedEnrollmentDecisionCommandTransaction(current map[string]string) map[string]string {
	if current[enrollmentDecisionCommandTransactionSite] != enrollmentDecisionCommandTransactionReason {
		return current
	}
	if _, kept := current[enrollmentDecisionHandlerTransactionSite]; kept {
		return current
	}
	remaining := maps.Clone(current)
	delete(remaining, enrollmentDecisionCommandTransactionSite)
	return remaining
}
