package auditlog

// ApprovedBooking is the Enrollment fact the booking consistency audit reads
// of one approved booking window. Dates are YYYY-MM-DD. The audit declares
// it, so the root binds Enrollment to it without the Audit Platform naming a
// domain owner (#4072).
type ApprovedBooking struct {
	RequestChildID            int64  `json:"request_child_id"`
	StudentID                 *int64 `json:"student_id"`
	PhaseID                   int64  `json:"phase_id"`
	TenantID                  int64  `json:"tenant_id"`
	ServiceStartDate          string `json:"service_start_date"`
	ServiceEndDate            string `json:"service_end_date"`
	CareOfferingSelectionMode string `json:"care_offering_selection_mode"`
}

// ApprovedBookingOfferingLink is the Enrollment fact the booking consistency
// audit reads of one offering an approved booking selected. Dates are
// YYYY-MM-DD.
type ApprovedBookingOfferingLink struct {
	ID             int64    `json:"id"`
	TenantID       int64    `json:"tenant_id"`
	RequestChildID int64    `json:"request_child_id"`
	CareOfferingID int64    `json:"care_offering_id"`
	SelectedDays   []string `json:"selected_days"`
	ValidFrom      *string  `json:"valid_from"`
	ValidUntil     *string  `json:"valid_until"`
}
