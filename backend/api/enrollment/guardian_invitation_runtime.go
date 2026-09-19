package enrollment

import "context"

// Issuing a guardian invitation is an Identity & Access flow (#2722).
// Approving the first child of a family fires one, so the decision routes
// reach the owner through the runtime below, which the composition root
// binds (#3332). The routes name no owner contract: the invitation is
// fire-and-forget here, and its failure is logged rather than answered, so
// the runtime needs no outcome beyond the error.
type GuardianInvitationRuntime struct {
	// Create issues the invitation for a guardian contact that has an
	// address on file and no account yet. The school comes from the tenant
	// in the context the caller passes.
	Create func(ctx context.Context, guardianProfileID, createdBy int64) error
}

func (rt *GuardianInvitationRuntime) configured() bool {
	return rt != nil && rt.Create != nil
}
