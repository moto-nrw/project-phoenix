package supervisiondashboard

import (
	"context"
	"fmt"
)

// loadTeacherHints attaches today's hints of the Lehrkraft (#4795). They are
// notes about the child, so the caller runs this behind the gate of the
// planning times (users:read and full access).
func (s *service) loadTeacherHints(ctx context.Context, projection *Projection, studentIDs []int64, businessDay Date) error {
	if s.deps.TeacherHints == nil {
		return nil
	}
	hints, err := s.deps.TeacherHints.TeacherHints(ctx, studentIDs, businessDay)
	if err != nil {
		return fmt.Errorf("load teacher hints: %w", err)
	}
	projection.TeacherHints = append(projection.TeacherHints, hints...)
	return nil
}
