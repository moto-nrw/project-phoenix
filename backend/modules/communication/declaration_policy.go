package communication

import "time"

// DeclarationPolicy evaluates the same declaration rules for both portals.
// Primitive facts let consumer-owned ports bind it without sharing owner DTOs.
type DeclarationPolicy struct{}

func (DeclarationPolicy) Closed(deadline *time.Time, now time.Time) bool {
	return (DeclarationRules{Deadline: deadline}).Closed(now)
}
func (DeclarationPolicy) ChildState(signers string, deadline *time.Time, eligible []int64, latest map[int64]string, now time.Time) string {
	return DeclarationChildState(DeclarationRules{Signers: signers, Deadline: deadline}, eligible, latest, now)
}
func (DeclarationPolicy) AllowedActions(revocable bool, deadline *time.Time, canSubmit bool, latest string, now time.Time) []string {
	return DeclarationAllowedActions(DeclarationRules{Revocable: revocable, Deadline: deadline}, canSubmit, latest, now)
}
func (DeclarationPolicy) OwesAction(deadline *time.Time, canSubmit bool, latest, state string, now time.Time) bool {
	return DeclarationOwesAction(DeclarationRules{Deadline: deadline}, canSubmit, latest, state, now)
}

func (DeclarationPolicy) ActionAllowed(revocable bool, deadline *time.Time, canSubmit bool, latest, action string, now time.Time) bool {
	return DeclarationActionAllowed(DeclarationRules{Revocable: revocable, Deadline: deadline}, canSubmit, latest, action, now)
}
