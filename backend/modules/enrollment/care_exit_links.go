package enrollment

import "context"

// CareExitApplicationLink supplies the student links and application status used
// to evaluate care-booking ownership. It contains no child or guardian names.
type CareExitApplicationLink struct {
	ID               int64  `json:"id"`
	TenantID         int64  `json:"tenant_id"`
	CreatedStudentID *int64 `json:"created_student_id"`
	MatchedStudentID *int64 `json:"matched_student_id"`
	Status           string `json:"status"`
}

// CareExitApplicationLinks returns links for the selected students. An empty
// selection supplies the school's links for the scheduled expiry evaluation.
func (m *Module) CareExitApplicationLinks(ctx context.Context, studentIDs []int64) ([]CareExitApplicationLink, error) {
	var links []CareExitApplicationLink
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		links, err = m.engine.CareExitApplicationLinks(txCtx, studentIDs)
		return err
	})
	return links, err
}
