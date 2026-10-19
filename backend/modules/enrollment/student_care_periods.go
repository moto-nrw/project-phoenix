package enrollment

import "context"

type StudentCarePeriod struct {
	RequestChildID    int64
	RequestID         int64
	PhaseID           int64
	PhaseName         string
	ServiceStartDate  Date
	ServiceEndDate    Date
	TargetGradeLevel  *int16
	TargetSchoolClass *string
}

// StudentCarePeriods returns approved applications that created the student.
// Matched-only applications are intentionally not part of this projection.
func (m *Module) StudentCarePeriods(ctx context.Context, studentID int64) ([]*StudentCarePeriod, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.StudentCarePeriods(ctx, studentID)
}
