package enrollment

import "context"

type StudentCarePeriod struct {
	RequestChildID   int64
	RequestID        int64
	PhaseID          int64
	PhaseName        string
	ServiceStartDate Date
	ServiceEndDate   Date
}

// StudentCarePeriods returns approved applications that created the student.
// Matched-only applications are intentionally not part of this projection.
func (m *Module) StudentCarePeriods(ctx context.Context, studentID int64) ([]*StudentCarePeriod, error) {
	var periods []*StudentCarePeriod
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		periods, err = m.engine.StudentCarePeriods(ctx, studentID)
		return err
	})
	return periods, err
}
