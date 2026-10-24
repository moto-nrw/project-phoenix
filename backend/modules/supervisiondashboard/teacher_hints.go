package supervisiondashboard

import (
	"context"
	"fmt"
)

// loadTeacherHints attaches today's hints of the Lehrkraft (#4795). They are
// notes about the child, so they stay behind the planning gate (users:read
// and full access), like the planning times.
func (s *service) loadTeacherHints(ctx context.Context, projection *Projection, caller Caller, studentIDs []int64, fullAccess bool, businessDay Date) error {
	if s.deps.TeacherHints == nil || !planningVisible(caller, studentIDs, fullAccess) {
		return nil
	}
	hints, err := s.deps.TeacherHints.TeacherHints(ctx, studentIDs, businessDay)
	if err != nil {
		return fmt.Errorf("load teacher hints: %w", err)
	}
	projection.TeacherHints = append(projection.TeacherHints, hints...)
	return nil
}
