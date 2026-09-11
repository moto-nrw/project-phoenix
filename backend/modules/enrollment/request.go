package enrollment

import (
	"encoding/json"
	"time"
)

// Request is a submission pinned to an immutable schema version.
type Request struct {
	ID                       int64           `json:"id"`
	TenantID                 int64           `json:"tenant_id"`
	CreatedAt                time.Time       `json:"created_at"`
	UpdatedAt                time.Time       `json:"updated_at"`
	SchemaID                 *int64          `json:"schema_id,omitempty"`
	PhaseID                  int64           `json:"phase_id"`
	GuardianFirstName        string          `json:"guardian_first_name"`
	GuardianLastName         string          `json:"guardian_last_name"`
	GuardianEmail            string          `json:"guardian_email"`
	GuardianPhone            *string         `json:"guardian_phone,omitempty"`
	GuardianAccountID        *int64          `json:"guardian_account_id,omitempty"`
	ConsentFlags             json.RawMessage `json:"consent_flags"`
	LegalBlocksSnapshot      json.RawMessage `json:"-"`
	CustomData               json.RawMessage `json:"custom_data"`
	SubmissionSource         string          `json:"submission_source"`
	SourceMetadata           json.RawMessage `json:"source_metadata"`
	StatusToken              string          `json:"status_token"`
	StatusTokenExpires       *time.Time      `json:"status_token_expires,omitempty"`
	SubmittedAt              time.Time       `json:"submitted_at"`
	WithdrawnAt              *time.Time      `json:"withdrawn_at,omitempty"`
	DecisionNotificationMode *string         `json:"-"`
}

type RequestListFilters struct {
	PhaseID           int64
	ChildStatus       string
	CreatedStudentID  int64
	CreatedStudentIDs []int64
}

// LegalBlockSnapshot is one resolved legal block exactly as the public
// form rendered it at (re)submission time. It mirrors the resolved view
// (services/enrollment.LegalBlock), not the template row, so the record
// stays meaningful for blocks that came from live tenant settings.
type LegalBlockSnapshot struct {
	Key      string `json:"key"`
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Label    string `json:"label"`
	Text     string `json:"text"`
	Required bool   `json:"required"`
	Source   string `json:"source,omitempty"`
}

// LegalBlocksSnapshotEntry records one parent-facing (re)submission:
// which legal blocks were shown, in which wording, which checkboxes the
// guardian ticked, at what time. ConsentFlags is the filtered flag map
// as persisted for that submission — without it a later edit that
// overwrites Request.ConsentFlags would erase the proof of the earlier
// answer (e.g. photo consent false → true). Entries are never rewritten
// or deleted while the request lives; they share the request's
// retention.
type LegalBlocksSnapshotEntry struct {
	SnapshotAt   time.Time            `json:"snapshot_at"`
	Blocks       []LegalBlockSnapshot `json:"blocks"`
	ConsentFlags json.RawMessage      `json:"consent_flags"`
}

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
