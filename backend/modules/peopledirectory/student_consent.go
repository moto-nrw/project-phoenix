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

func (m *Module) CurrentStudentConsents(
	ctx context.Context,
	snapshot StudentConsentSnapshot,
	canManagePhoto bool,
) ([]StudentConsentState, error) {
	if snapshot.StudentID <= 0 {
		return nil, invalidStudent("a persisted student is required")
	}
	return m.engine.CurrentStudentConsents(ctx, snapshot, canManagePhoto)
}
