package enrollment

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Request is a row in enrollment.requests - one parent submission. Each
// row carries the immutable schema_id pinning it to the form-schema
// version that was active at submission time.
//
// Status is derived from child statuses (see RequestStatus values) and
// computed by the request service, not stored.
type Request struct {
	base.Model `bun:"schema:enrollment,table:requests"`
	base.TenantModel
	// SchemaID is nullable: a phase set to "Basis" with no tenant-
	// active schema submits without one. The form_schemas FK still
	// applies when set, so admins can audit which schema version a
	// non-trivial submission was bound to.
	SchemaID           *int64         `bun:"schema_id" json:"schema_id,omitempty"`
	PhaseID            int64          `bun:"phase_id,notnull" json:"phase_id"`
	GuardianFirstName  string         `bun:"guardian_first_name,notnull" json:"guardian_first_name"`
	GuardianLastName   string         `bun:"guardian_last_name,notnull" json:"guardian_last_name"`
	GuardianEmail      string         `bun:"guardian_email,notnull" json:"guardian_email"`
	GuardianPhone      *string        `bun:"guardian_phone" json:"guardian_phone,omitempty"`
	GuardianAccountID  *int64         `bun:"guardian_account_id" json:"guardian_account_id,omitempty"`
	ConsentFlags       map[string]any `bun:"consent_flags,type:jsonb,notnull,default:'{}'" json:"consent_flags"`
	CustomData         map[string]any `bun:"custom_data,type:jsonb,notnull,default:'{}'" json:"custom_data"`
	SubmissionSource   string         `bun:"submission_source,notnull,default:'public'" json:"submission_source"`
	SourceMetadata     map[string]any `bun:"source_metadata,type:jsonb,notnull,default:'{}'" json:"source_metadata"`
	StatusToken        string         `bun:"status_token,notnull,unique" json:"status_token"`
	StatusTokenExpires *time.Time     `bun:"status_token_expires" json:"status_token_expires,omitempty"`
	SubmittedAt        time.Time      `bun:"submitted_at,notnull,default:current_timestamp" json:"submitted_at"`
	WithdrawnAt        *time.Time     `bun:"withdrawn_at" json:"withdrawn_at,omitempty"`

	// DecisionNotificationMode is pinned when the first parent-notifiable
	// decision is made. It is internal state, not part of the request API.
	DecisionNotificationMode *string `bun:"decision_notification_mode" json:"-"`
}

// Consent-flag keys stored in Request.ConsentFlags. These are the
// canonical string keys shared across the submit validation, the
// decision service (which stamps the matching student consent
// timestamps), and the public form.
//
// KEEP IN SYNC with the consent_flags object built in
// frontend/src/components/enrollment/enrollment-form.tsx.
const (
	ConsentKeyAGB            = "agb"
	ConsentKeyDataProcessing = "data_processing"
	ConsentKeyEmailContact   = "email_contact"
	ConsentKeyPhoto          = "photo"
)

// RequestStatus values - derived, not stored. Documented here as the
// canonical source for the derivation logic the request service applies.
const (
	RequestStatusSubmitted   = "submitted"    // every child still submitted
	RequestStatusUnderReview = "under_review" // at least one child under_review, no decisions yet
	RequestStatusPartial     = "partial"      // some children decided, others pending
	RequestStatusFinalized   = "finalized"    // all children in a terminal status
	RequestStatusWithdrawn   = "withdrawn"    // request withdrawn (withdrawn_at set)
)

const (
	RequestSourcePublic      = "public"
	RequestSourceLateInvite  = "late_invite"
	RequestSourceAdminManual = "admin_manual"
)

// RequestListFilters narrows the admin list query. Zero-value fields
// are ignored. ChildStatus matches when ANY child of the request
// carries the given status - handy for "show me everything still
// awaiting a decision".
type RequestListFilters struct {
	PhaseID           int64
	ChildStatus       string
	CreatedStudentID  int64
	CreatedStudentIDs []int64
}

// DuplicateChildKey identifies one (first_name, last_name) pair the
// caller wants to dedup against existing enrollments in a phase.
// Comparison is case-insensitive, with leading/trailing whitespace
// trimmed at the SQL layer.
type DuplicateChildKey struct {
	FirstName string
	LastName  string
}

// RequestRepository describes the DB operations PR 5/7/8 need.
type RequestRepository interface {
	Create(ctx context.Context, req *Request) error
	FindByID(ctx context.Context, id int64) (*Request, error)
	ListByIDs(ctx context.Context, ids []int64) ([]*Request, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*Request, error)
	FindByStatusToken(ctx context.Context, token string) (*Request, error)
	FindByStatusTokenForUpdate(ctx context.Context, token string) (*Request, error)
	AcquireSubmissionDedupLock(ctx context.Context, phaseID int64, emailHash uint64) error
	// AcquireExistingStudentMatchLock serializes existing-student
	// re-enrollment matching for a phase across all guardians (the
	// email-scoped dedup lock does not, so different-email submissions can
	// otherwise both pin the same already-enrolled student). Held until the
	// caller's transaction ends.
	AcquireExistingStudentMatchLock(ctx context.Context, phaseID int64) error
	// HasActiveRequestForMatchedStudent reports whether any non-rejected,
	// non-withdrawn request_child in the phase is already pinned to the given
	// already-enrolled student. excludeRequestChildID (0 = none) skips one
	// persisted row so a re-check of an already-active row does not find
	// itself.
	HasActiveRequestForMatchedStudent(ctx context.Context, phaseID, studentID, excludeRequestChildID int64) (bool, error)
	// PinDecisionNotificationMode atomically stores proposed only while the
	// request is still unpinned and always returns the effective stored mode.
	PinDecisionNotificationMode(ctx context.Context, requestID int64, proposed string) (string, error)

	// ListAdmin returns every request matching the filters, newest
	// first. PR 8's admin review UI consumes this; the parent-facing
	// flows never call it.
	ListAdmin(ctx context.Context, filters RequestListFilters) ([]*Request, error)

	// UpdateGuardianData writes the guardian-editable fields (names, phone,
	// consent flags, custom answers) and bumps updated_at.
	UpdateGuardianData(ctx context.Context, req *Request) error
	UpdateGuardianDataWithEmail(ctx context.Context, req *Request) error

	// MarkWithdrawn stamps withdrawn_at and bumps updated_at.
	MarkWithdrawn(ctx context.Context, requestID int64, withdrawnAt time.Time) error

	// ClearWithdrawn is the inverse of MarkWithdrawn: nulls withdrawn_at and
	// bumps updated_at. Used by the admin restore flow (#2157).
	ClearWithdrawn(ctx context.Context, requestID int64) error

	// FindActiveDuplicate returns the names of any children for which a
	// non-terminal-rejected/withdrawn enrollment already exists for the
	// same (phase_id, guardian_email). Empty result means safe to
	// proceed. Used by the submit flow to block accidental
	// double-submits without affecting different parents or different
	// child names.
	FindActiveDuplicate(ctx context.Context, phaseID int64, guardianEmail string, children []DuplicateChildKey) ([]DuplicateChildKey, error)
	FindActiveDuplicateExcludingRequest(ctx context.Context, phaseID int64, guardianEmail string, children []DuplicateChildKey, excludedRequestID int64) ([]DuplicateChildKey, error)

	// ExistsByPhaseID reports whether any request row references the
	// given phase.
	ExistsByPhaseID(ctx context.Context, phaseID int64) (bool, error)

	// CountByPhaseID returns how many request rows reference the phase.
	// Powers the phase-delete confirmation modal.
	CountByPhaseID(ctx context.Context, phaseID int64) (int, error)

	// DeleteByPhaseID removes every request for the phase (cascading to
	// request_children + request_child_offerings) and returns the number
	// of requests deleted. Created students are preserved — the
	// created_student_id FK is ON DELETE SET NULL. Must run before the
	// phase's care offerings are deleted because of the
	// request_child_offerings.care_offering_id RESTRICT FK.
	DeleteByPhaseID(ctx context.Context, phaseID int64) (int, error)
	ListFullyRejectedBefore(ctx context.Context, cutoff time.Time) ([]int64, error)
	DeleteByID(ctx context.Context, requestID int64) error

	// ExistsBySchemaID reports whether any request row references the
	// given schema version. The schema delete path uses this to preserve
	// historical submissions.
	ExistsBySchemaID(ctx context.Context, schemaID int64) (bool, error)
}
