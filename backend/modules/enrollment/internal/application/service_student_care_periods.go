package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// StudentCarePeriods returns approved applications that created the student.
// Matched-only applications are intentionally not part of this projection.
func (m *Service) StudentCarePeriods(ctx context.Context, studentID int64) ([]*enrollment.StudentCarePeriod, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.StudentCarePeriods(ctx, studentID)
}
