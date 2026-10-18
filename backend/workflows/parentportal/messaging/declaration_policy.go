package messaging

import "time"

// DeclarationPolicy interprets Communication's declaration facts. The root
// binds the public owner policy; the workflow keeps authorization and writes.
type DeclarationPolicy interface {
	CanonicalRecord(DeclarationSubmission) []byte
	CanonicalContent(DeclarationVersion) []byte
	Closed(deadline *time.Time, now time.Time) bool
	ChildState(signers string, deadline *time.Time, eligible []int64, latest map[int64]string, now time.Time) string
	AllowedActions(revocable bool, deadline *time.Time, canSubmit bool, latest string, now time.Time) []string
	ActionAllowed(revocable bool, deadline *time.Time, canSubmit bool, latest, action string, now time.Time) bool
	OwesAction(deadline *time.Time, canSubmit bool, latest, state string, now time.Time) bool
}

type declarationRules struct {
	Signers   string
	Revocable bool
	Deadline  *time.Time
}
