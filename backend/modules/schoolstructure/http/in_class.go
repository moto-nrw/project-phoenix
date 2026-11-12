package schoolstructurehttp

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// InClassDay reports whether a child checked out with "Geht zum Unterricht"
// still reads "Im Unterricht" now. The day-planning owner decides it from the
// school's care day end; the root binds that rule, so this list answers like
// the staff views, which read "Zuhause" after the care day (#5656, #5710).
type InClassDay interface {
	InClassStillHolds(ctx context.Context) (bool, error)
}

// endInClassAfterCareDay turns "Im Unterricht" into "Abwesend" once the care
// day is over. It asks the rule only when a child is in class.
func (rs *Resource) endInClassAfterCareDay(ctx context.Context, responses []GroupStudentResponse) error {
	holds := true
	asked := false
	for i := range responses {
		if responses[i].Location != studentpresence.InClassLocationLabel {
			continue
		}
		if !asked {
			var err error
			if holds, err = rs.InClass.InClassStillHolds(ctx); err != nil {
				return err
			}
			asked = true
		}
		if !holds {
			responses[i].Location = studentpresence.AbsentLocationLabel
		}
	}
	return nil
}
