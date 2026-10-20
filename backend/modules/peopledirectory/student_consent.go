package peopledirectory

import (
	"context"
	"time"
)

// Consent keys of the four acknowledgements recorded on a child. They are the
// stored discriminators of the consent trail, so both portals name the same
// consent by the same key.
const (
	StudentConsentAGB            = "agb"
	StudentConsentDataProcessing = "data_processing"
	StudentConsentEmailContact   = "email_contact"
	StudentConsentPhoto          = "photo"
)

// Consent states as the staff and parent portals render them.
const (
	StudentConsentStateGranted     = "granted"
	StudentConsentStateWithdrawn   = "withdrawn"
	StudentConsentStateNotRecorded = "not_recorded"
)

// StudentConsentSnapshot is the child's four live consent timestamps. The
// caller passes the row it already read, so resolving the shared state costs
// no extra directory query.
type StudentConsentSnapshot struct {
	StudentID                int64
	AGBAcceptedAt            *time.Time
	DataProcessingAcceptedAt *time.Time
	EmailContactAcceptedAt   *time.Time
	PhotoConsentGivenAt      *time.Time
}

// StudentConsentState is the current staff/parent-facing state of one consent
// or required acknowledgement. ChangedAt is the grant/withdrawal time when
// known. CanWithdraw and CanGrant are decided by the calling portal's
// permission check and the recorded state.
type StudentConsentState struct {
	Key         string     `json:"key"`
	State       string     `json:"state"`
	ChangedAt   *time.Time `json:"changed_at,omitempty"`
	CanWithdraw bool       `json:"can_withdraw"`
	CanGrant    bool       `json:"can_grant"`
}

// StudentConsentQuery resolves the four live consent timestamps and the latest
// photo withdrawal into one shared projection. Both portals use this, so staff
// cannot see a different state than parents.
type StudentConsentQuery interface {
	// CurrentStudentConsents returns the four states in a stable order.
	// canManagePhoto decides whether the photo consent is offered as
	// withdrawable (granted) or grantable (withdrawn).
	CurrentStudentConsents(ctx context.Context, snapshot StudentConsentSnapshot, canManagePhoto bool) ([]StudentConsentState, error)
}

// StudentConsentTransition is one consent whose timestamp appeared or
// disappeared between two snapshots of a child.
type StudentConsentTransition struct {
	Key string
	// Granted is false for a withdrawal.
	Granted bool
	// At is the new timestamp of a grant and the time of the change for a
	// withdrawal.
	At time.Time
}

// StudentConsentTransitions lists the consents that were granted or
// withdrawn between before and after, in the key order of the states. A
// timestamp that stayed set, even to another value, or stayed unset is no
// transition.
func StudentConsentTransitions(before, after StudentConsentSnapshot, changedAt time.Time) []StudentConsentTransition {
	fields := []struct {
		key           string
		before, after *time.Time
	}{
		{StudentConsentAGB, before.AGBAcceptedAt, after.AGBAcceptedAt},
		{StudentConsentDataProcessing, before.DataProcessingAcceptedAt, after.DataProcessingAcceptedAt},
		{StudentConsentEmailContact, before.EmailContactAcceptedAt, after.EmailContactAcceptedAt},
		{StudentConsentPhoto, before.PhotoConsentGivenAt, after.PhotoConsentGivenAt},
	}
	transitions := make([]StudentConsentTransition, 0, len(fields))
	for _, field := range fields {
		if (field.before != nil) == (field.after != nil) {
			continue
		}
		transition := StudentConsentTransition{Key: field.key, At: changedAt}
		if field.after != nil {
			transition.Granted, transition.At = true, *field.after
		}
		transitions = append(transitions, transition)
	}
	return transitions
}

// StudentConsentEvent is one recorded consent change of a child.
type StudentConsentEvent struct {
	Key       string
	Withdrawn bool
	At        time.Time
}

// PhotoConsentWithdrawnAt is when the photo consent was withdrawn, read from
// the events newest first: only a withdrawal as the newest photo event
// counts, since a later grant means the consent stands.
func PhotoConsentWithdrawnAt(events []StudentConsentEvent) *time.Time {
	for _, event := range events {
		if event.Key != StudentConsentPhoto {
			continue
		}
		if !event.Withdrawn {
			return nil
		}
		withdrawnAt := event.At
		return &withdrawnAt
	}
	return nil
}
