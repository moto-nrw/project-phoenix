package schedulerecords

import (
	"errors"
	"time"
)

// ErrCareRequestNotPending means a pending-row transition lost a race or the
// row was already terminal under the caller's tenant.
var ErrCareRequestNotPending = errors.New("schedule: care schedule change request is not pending")

// ErrCareRequestNotDecided means a correction was attempted on a row that
// carries no decision to correct — still pending, or closed some other way
// (withdrawn, care ended, marked done).
var ErrCareRequestNotDecided = errors.New("schedule: care schedule change request is not decided")

// ErrCareRequestNotFound means no row with the requested id exists in the
// caller's tenant.
var ErrCareRequestNotFound = errors.New("schedule: care schedule change request not found")

// Care-schedule change-request lifecycle states. A guardian submits a pending
// request; a staff decision moves it to approved (and applies the weekly plan)
// or rejected; the guardian can withdraw it while still pending.
const (
	CareRequestStatusPending   = "pending"
	CareRequestStatusApproved  = "approved"
	CareRequestStatusRejected  = "rejected"
	CareRequestStatusWithdrawn = "withdrawn"
	// CareRequestStatusDone closes a request that only covered days already
	// gone: nothing to apply, and "abgelehnt" would misstate what happened.
	CareRequestStatusDone = "done"
	// CareRequestStatusCareEnded closes an open request whose child left the
	// OGS before anybody decided it (#2487).
	CareRequestStatusCareEnded = "care_ended"
)

const (
	CareRequestKindWeeklySchedule = "weekly_schedule"
	CareRequestKindPickupChange   = "pickup_change"
)

// CareScheduleChangeRequest is one parent-initiated change to a child's
// recurring weekly care plan (departure mode + arrival/pickup times per
// weekday). One row per request carrying the whole canonical payload
// {"weekdays":[{"weekday","mode","arrival","pickup"}]} — the request is
// decided atomically, and apply merges only the filled aspects onto the live
// plan. At most one pending request exists per student (partial unique index).
type CareScheduleChangeRequest struct {
	Model `bun:"schema:schedule,table:care_schedule_change_requests"`
	TenantModel

	StudentID      int64          `bun:"student_id,notnull" json:"student_id"`
	SubmittedBy    int64          `bun:"submitted_by,notnull" json:"submitted_by"`
	RequestKind    string         `bun:"request_kind,notnull,default:'weekly_schedule'" json:"request_kind"`
	Payload        map[string]any `bun:"payload,type:jsonb,notnull" json:"payload"`
	Status         string         `bun:"status,notnull,default:'pending'" json:"status"`
	DecisionReason *string        `bun:"decision_reason" json:"decision_reason,omitempty"`
	ReviewedBy     *int64         `bun:"reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt     *time.Time     `bun:"reviewed_at" json:"reviewed_at,omitempty"`
	AppliedAt      *time.Time     `bun:"applied_at" json:"applied_at,omitempty"`
	// DecisionSnapshot freezes the "alt → neu" diff the reviewer saw when
	// deciding (ADR 0002, #2430). NULL on pending, withdrawn, and pre-#2430
	// rows — history readers fall back to the payload-derived requested
	// summary there.
	DecisionSnapshot *CareRequestDecisionSnapshot `bun:"decision_snapshot,type:jsonb" json:"decision_snapshot,omitempty"`
}

// CareRequestDecisionSnapshot is the retained repository's persisted JSON shape.
type CareRequestDecisionSnapshot struct {
	Diff []CareRequestSnapshotEntry `json:"diff"`
}

type CareRequestSnapshotEntry struct {
	Label    string   `json:"label"`
	Old      string   `json:"old,omitempty"`
	New      string   `json:"new,omitempty"`
	Weekday  int      `json:"weekday,omitempty"`
	CareKind string   `json:"care_kind,omitempty"`
	OldModes []string `json:"old_modes,omitempty"`
	NewMode  string   `json:"new_mode,omitempty"`
}

// ports.CareScheduleChangeRequestRecords is the tenant-scoped data-access
// contract. All methods MUST run inside a tenant transaction (the parent
// service resolves the child's tenant first, then wraps the call in
// tenant.WithTenantTx); the staff review queue runs under the request's
// tenant middleware.
