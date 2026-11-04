package studentpresence

import (
	"context"
	"time"
)

// PresentStudentCounts answers the Träger's view of a school (#5686): how many
// children are present right now, by the rule of the school's own dashboard:
// an open attendance of the day or an open room visit, one count per child.
// A school without a present child is missing from the map.
//
// The caller must run it in the administrative transaction; it reads only the
// given schools.
type PresentStudentCounts interface {
	CountPresentStudentsByTenant(ctx context.Context, schools []int64, at time.Time) (map[int64]int, error)
}
