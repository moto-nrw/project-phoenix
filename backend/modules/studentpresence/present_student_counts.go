package studentpresence

import (
	"context"
	"time"
)

// PresentStudents answers the Träger's view of a school (#5686): which
// children are present right now, by the rule of the school's own dashboard:
// an open attendance of the day or an open room visit, once per child. A
// school without a present child is missing from the map.
//
// The caller must run it in the administrative transaction; it reads only the
// given schools.
type PresentStudents interface {
	PresentStudentIDsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64][]int64, error)
}
