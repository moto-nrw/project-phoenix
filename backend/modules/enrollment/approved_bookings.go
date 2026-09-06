package enrollment

import "context"

// ApprovedBooking supplies the Enrollment facts needed by booking consistency
// audits. Student lifecycle and care coverage remain with their respective owners.
type ApprovedBooking struct {
	RequestChildID            int64  `json:"request_child_id"`
	StudentID                 *int64 `json:"student_id"`
	PhaseID                   int64  `json:"phase_id"`
	TenantID                  int64  `json:"tenant_id"`
	ServiceStartDate          Date   `json:"service_start_date"`
	ServiceEndDate            Date   `json:"service_end_date"`
	CareOfferingSelectionMode string `json:"care_offering_selection_mode"`
}

func (m *Module) ApprovedBookings(ctx context.Context) ([]ApprovedBooking, error) {
	var rows []ApprovedBooking
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		rows, err = m.engine.ApprovedBookings(txCtx)
		return err
	})
	return rows, err
}
