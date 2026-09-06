package enrollment

import "time"

// OfferingGradeCount counts distinct children in a grade bucket. Nil is the
// unknown-grade bucket, which must not disappear from availability warnings.
type OfferingGradeCount struct {
	CareOfferingID int64
	GradeLevel     *int16
	Count          int
}

// RequestChildOffering records a child selection and its half-open validity interval.
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
	ValidFrom  *Date `json:"valid_from,omitempty"`
	ValidUntil *Date `json:"valid_until,omitempty"`
}

// ApprovedOfferingSelection resolves Enrollment's child-to-student reference.
// Student status and class must be obtained from People Directory.
type ApprovedOfferingSelection struct {
	Selection *RequestChildOffering
	StudentID int64
}
