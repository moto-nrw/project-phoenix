package enrollment

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// RequestChild is the legacy service value for a child in a parent submission.
// The Enrollment owner persists it through its child adapter. Each child has
// an independent status; the parent request's status is derived from its children.
type RequestChild struct {
	ID               int64         `json:"id"`
	TenantID         int64         `json:"tenant_id"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
	RequestID        int64         `json:"request_id"`
	FirstName        string        `json:"first_name"`
	LastName         string        `json:"last_name"`
	DateOfBirth      timezone.Date `json:"date_of_birth"`
	TargetGradeLevel *int16        `json:"target_grade_level,omitempty"`
	// TargetSchoolClass is the concrete future class (e.g. "2a") chosen at
	// enrollment (migration 1.15.172, issue #1833). NULL/empty means
	// grade-only ("Klasse offen"); on approval a non-empty value lands
	// verbatim in users.students.school_class, otherwise the grade-derived
	// fallback is used. Only collected for grade >= 2 when the tenant
	// setting enrollment.collect_school_class is on.
	TargetSchoolClass *string        `json:"target_school_class,omitempty"`
	CustomData        map[string]any `json:"custom_data"`
	Status            string         `json:"status"`
	StatusReason      *string        `json:"status_reason,omitempty"`
	ActivationMode    string         `json:"activation_mode"`
	ActivateOn        *timezone.Date `json:"activate_on,omitempty"`
	ReviewedAt        *time.Time     `json:"reviewed_at,omitempty"`
	ReviewedBy        *int64         `json:"reviewed_by,omitempty"`
	CreatedStudentID  *int64         `json:"created_student_id,omitempty"`
	// MatchedStudentID (migration 1.15.221) — set only for existing_students
	// phases: the already-enrolled student this child was matched to at
	// submission (unambiguous name+birthday lookup). On approval the decision
	// service renews that student instead of creating a duplicate
	// Person/Student. NULL for every other audience and when the submission
	// matched zero or more than one enrolled student (ambiguous → left to the
	// fresh-create path). FK ON DELETE SET NULL: a student deleted before
	// approval clears the reference.
	MatchedStudentID *int64 `json:"matched_student_id,omitempty"`
	SortOrder        int    `json:"sort_order"`

	// Rollover columns (migration 1.15.62). NULL on rows created via
	// the public form; set by RolloverService when a previous-year
	// approved child is carried forward into a new phase.
	//
	// RolloverSourceChildID — the previous-year row this one was
	// derived from. Unique index ensures one-to-one mapping.
	// ReviewReason — populated only when Status == pending_admin_review;
	// drives the localised label in the admin review queue.
	RolloverSourceChildID *int64  `json:"rollover_source_child_id,omitempty"`
	ReviewReason          *string `json:"review_reason,omitempty"`
}

// RequestChildOffering is a row in enrollment.request_child_offerings -
// the join table linking a request_child to a care_offering. PR 6
// ships the schema; PR 7 fills it on submission.
type RequestChildOffering struct {
	ID                    int64     `json:"id"`
	TenantID              int64     `json:"tenant_id"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
	RequestChildID        int64     `json:"request_child_id"`
	CareOfferingID        int64     `json:"care_offering_id"`
	SelectedDays          []string  `json:"selected_days,omitempty"`
	ManualSelectedDays    []string  `json:"manual_selected_days,omitempty"`
	AutomaticSelectedDays []string  `json:"automatic_selected_days,omitempty"`
	Notes                 *string   `json:"notes,omitempty"`
	// ValidFrom / ValidUntil make an approved offering switch effective on its
	// requested date. ValidUntil is exclusive, matching student enrollments.
	ValidFrom  *timezone.Date `json:"valid_from,omitempty"`
	ValidUntil *timezone.Date `json:"valid_until,omitempty"`
}

// ApprovedOfferingChild is one approved, still-relevant offering selection
// with the enrollment-to-student resolution the offering-source flows need
// (#2137): roster resync of sourced Regeltermine and the editor's per-grade
// count preview.
type ApprovedOfferingChild struct {
	Link        *RequestChildOffering
	StudentID   int64
	SchoolClass string
}
