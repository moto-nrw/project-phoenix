package domain

import "time"

// Consent keys of the four acknowledgements recorded on a child.
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

// StudentConsentSnapshot is the child's four live consent timestamps.
type StudentConsentSnapshot struct {
	StudentID                int64
	AGBAcceptedAt            *time.Time
	DataProcessingAcceptedAt *time.Time
	EmailContactAcceptedAt   *time.Time
	PhotoConsentGivenAt      *time.Time
}

// StudentConsentState is the current state of one consent.
type StudentConsentState struct {
	Key         string
	State       string
	ChangedAt   *time.Time
	CanWithdraw bool
	CanGrant    bool
}

// CurrentStudentConsents folds the live timestamps into the shared portal
// projection. photoWithdrawnAt is the moment the latest photo consent was
// withdrawn, or nil when the trail records no withdrawal as the latest event.
//
// The three enrollment acknowledgements are never withdrawable: they are
// preconditions of the booking, not choices a portal may take back.
func CurrentStudentConsents(
	snapshot StudentConsentSnapshot,
	canManagePhoto bool,
	photoWithdrawnAt *time.Time,
) []StudentConsentState {
	photo := consentFromTimestamp(StudentConsentPhoto, snapshot.PhotoConsentGivenAt, canManagePhoto)
	if snapshot.PhotoConsentGivenAt == nil && photoWithdrawnAt != nil {
		withdrawnAt := *photoWithdrawnAt
		photo = StudentConsentState{
			Key:       StudentConsentPhoto,
			State:     StudentConsentStateWithdrawn,
			ChangedAt: &withdrawnAt,
			CanGrant:  canManagePhoto,
		}
	}
	return []StudentConsentState{
		consentFromTimestamp(StudentConsentAGB, snapshot.AGBAcceptedAt, false),
		consentFromTimestamp(StudentConsentDataProcessing, snapshot.DataProcessingAcceptedAt, false),
		consentFromTimestamp(StudentConsentEmailContact, snapshot.EmailContactAcceptedAt, false),
		photo,
	}
}

func consentFromTimestamp(key string, recordedAt *time.Time, canWithdraw bool) StudentConsentState {
	if recordedAt == nil {
		return StudentConsentState{Key: key, State: StudentConsentStateNotRecorded}
	}
	return StudentConsentState{
		Key:         key,
		State:       StudentConsentStateGranted,
		ChangedAt:   recordedAt,
		CanWithdraw: canWithdraw,
	}
}
