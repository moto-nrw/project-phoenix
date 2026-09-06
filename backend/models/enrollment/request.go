package enrollment

import (
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Request is the legacy service value for one parent submission. The
// Enrollment owner persists it through its request adapter. It carries the immutable schema_id pinning it to the form-schema
// version that was active at submission time.
//
// Status is derived from child statuses (see RequestStatus values) and
// computed by the request service, not stored.
type Request struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// SchemaID is nullable: a phase set to "Basis" with no tenant-
	// active schema submits without one. The form_schemas FK still
	// applies when set, so admins can audit which schema version a
	// non-trivial submission was bound to.
	SchemaID          *int64         `json:"schema_id,omitempty"`
	PhaseID           int64          `json:"phase_id"`
	GuardianFirstName string         `json:"guardian_first_name"`
	GuardianLastName  string         `json:"guardian_last_name"`
	GuardianEmail     string         `json:"guardian_email"`
	GuardianPhone     *string        `json:"guardian_phone,omitempty"`
	GuardianAccountID *int64         `json:"guardian_account_id,omitempty"`
	ConsentFlags      map[string]any `json:"consent_flags"`
	// LegalBlocksSnapshot is append-only consent evidence (Art. 5 Abs. 2,
	// Art. 7 Abs. 1 DSGVO): one entry per parent-facing (re)submission,
	// recording the resolved legal blocks exactly as the form rendered
	// them. Settings-sourced blocks are not versioned anywhere else, so
	// without this record a later settings edit would silently change
	// what a stored request "accepted". Not part of the request API yet
	// (deliberate, like DecisionNotificationMode) — admin-facing display
	// is a follow-up.
	LegalBlocksSnapshot []capability.LegalBlocksSnapshotEntry `json:"-"`
	CustomData          map[string]any                        `json:"custom_data"`
	SubmissionSource    string                                `json:"submission_source"`
	SourceMetadata      map[string]any                        `json:"source_metadata"`
	StatusToken         string                                `json:"status_token"`
	StatusTokenExpires  *time.Time                            `json:"status_token_expires,omitempty"`
	SubmittedAt         time.Time                             `json:"submitted_at"`
	WithdrawnAt         *time.Time                            `json:"withdrawn_at,omitempty"`

	// DecisionNotificationMode is pinned when the first parent-notifiable
	// decision is made. It is internal state, not part of the request API.
	DecisionNotificationMode *string `json:"-"`
}

// Consent-flag keys stored in Request.ConsentFlags. These are the
// canonical string keys shared across the submit validation, the
// decision service (which stamps the matching student consent
// timestamps), and the public form.
//
// KEEP IN SYNC with the consent_flags object built in
// frontend/src/components/enrollment/enrollment-form.tsx.
const (
	ConsentKeyAGB            = capability.ConsentKeyAGB
	ConsentKeyDataProcessing = capability.ConsentKeyDataProcessing
	ConsentKeyEmailContact   = capability.ConsentKeyEmailContact
	ConsentKeyPhoto          = capability.ConsentKeyPhoto
)

// RequestStatus values - derived, not stored. Documented here as the
// canonical source for the derivation logic the request service applies.
const (
	RequestStatusSubmitted   = capability.RequestStatusSubmitted   // every child still submitted
	RequestStatusUnderReview = capability.RequestStatusUnderReview // at least one child under_review, no decisions yet
	RequestStatusPartial     = capability.RequestStatusPartial     // some children decided, others pending
	RequestStatusFinalized   = capability.RequestStatusFinalized   // all children in a terminal status
	RequestStatusWithdrawn   = capability.RequestStatusWithdrawn   // request withdrawn (withdrawn_at set)
)

const (
	RequestSourcePublic      = capability.RequestSourcePublic
	RequestSourceLateInvite  = capability.RequestSourceLateInvite
	RequestSourceAdminManual = capability.RequestSourceAdminManual
)

// RequestListFilters narrows the admin list query. Zero-value fields
// are ignored. ChildStatus matches when ANY child of the request
// carries the given status - handy for "show me everything still
// awaiting a decision".
type RequestListFilters = capability.RequestListFilters

// DuplicateChildKey identifies one (first_name, last_name) pair the
// caller wants to dedup against existing enrollments in a phase.
// Comparison is case-insensitive, with leading/trailing whitespace
// trimmed at the SQL layer.
type DuplicateChildKey = capability.DuplicateChildKey
