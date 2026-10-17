package communication

import (
	"time"
)

type PortalDeclarationPolicy interface {
	PortalCanonicalRecord(pA PortalDeclarationSubmission) []byte
	PortalCanonicalContent(pA PortalDeclarationVersion) []byte
	Closed(deadline *time.Time, now time.Time) bool
	ChildState(signers string, deadline *time.Time, eligible []int64, latest map[int64]string, now time.Time) string
	AllowedActions(revocable bool, deadline *time.Time, canSubmit bool, latest string, now time.Time) []string
	ActionAllowed(revocable bool, deadline *time.Time, canSubmit bool, latest string, action string, now time.Time) bool
	OwesAction(deadline *time.Time, canSubmit bool, latest string, state string, now time.Time) bool
}
