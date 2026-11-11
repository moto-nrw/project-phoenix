package enrollment

// CareExitApplicationLink supplies the student links and application status used
// to evaluate care-booking ownership. It contains no child or guardian names.
type CareExitApplicationLink struct {
	ID               int64  `json:"id"`
	TenantID         int64  `json:"tenant_id"`
	CreatedStudentID *int64 `json:"created_student_id"`
	MatchedStudentID *int64 `json:"matched_student_id"`
	Status           string `json:"status"`
	// PhaseID is the phase of the child's request; the care exit records
	// the phase's bounds with the bookings it takes (#6657).
	PhaseID int64 `json:"phase_id"`
}

// IsApproved reports whether the application was approved.
func (l CareExitApplicationLink) IsApproved() bool { return l.Status == ChildStatusApproved }
