package parentportal

import (
	"time"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

type DeclarationPolicy interface{ messaging.DeclarationPolicy }
type DeclarationPolicyCallbacks struct {
	CanonicalRecord  func(pA DeclarationSubmission) []byte
	CanonicalContent func(pA DeclarationVersion) []byte
	Closed           func(deadline *time.Time, now time.Time) bool
	ChildState       func(signers string, deadline *time.Time, eligible []int64, latest map[int64]string, now time.Time) string
	AllowedActions   func(revocable bool, deadline *time.Time, canSubmit bool, latest string, now time.Time) []string
	ActionAllowed    func(revocable bool, deadline *time.Time, canSubmit bool, latest string, action string, now time.Time) bool
	OwesAction       func(deadline *time.Time, canSubmit bool, latest string, state string, now time.Time) bool
}

func NewDeclarationPolicy(c DeclarationPolicyCallbacks) DeclarationPolicy {
	return declarationPolicyBinding{c}
}

type declarationPolicyBinding struct{ callbacks DeclarationPolicyCallbacks }

func (p declarationPolicyBinding) CanonicalRecord(pA messaging.DeclarationSubmission) []byte {
	nativePA := messagingFromAppDeclarationSubmission(pA)
	result0 := p.callbacks.CanonicalRecord(nativePA)
	return result0
}
func (p declarationPolicyBinding) CanonicalContent(pA messaging.DeclarationVersion) []byte {
	nativePA := messagingFromAppDeclarationVersion(pA)
	result0 := p.callbacks.CanonicalContent(nativePA)
	return result0
}
func (p declarationPolicyBinding) Closed(deadline *time.Time, now time.Time) bool {
	result0 := p.callbacks.Closed(deadline, now)
	return result0
}
func (p declarationPolicyBinding) ChildState(signers string, deadline *time.Time, eligible []int64, latest map[int64]string, now time.Time) string {
	result0 := p.callbacks.ChildState(signers, deadline, eligible, latest, now)
	return result0
}
func (p declarationPolicyBinding) AllowedActions(revocable bool, deadline *time.Time, canSubmit bool, latest string, now time.Time) []string {
	result0 := p.callbacks.AllowedActions(revocable, deadline, canSubmit, latest, now)
	return result0
}
func (p declarationPolicyBinding) ActionAllowed(revocable bool, deadline *time.Time, canSubmit bool, latest string, action string, now time.Time) bool {
	result0 := p.callbacks.ActionAllowed(revocable, deadline, canSubmit, latest, action, now)
	return result0
}
func (p declarationPolicyBinding) OwesAction(deadline *time.Time, canSubmit bool, latest string, state string, now time.Time) bool {
	result0 := p.callbacks.OwesAction(deadline, canSubmit, latest, state, now)
	return result0
}
