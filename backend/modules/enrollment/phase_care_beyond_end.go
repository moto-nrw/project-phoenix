package enrollment

import (
	"context"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// PhaseUpdateReport is what a phase edit tells the school beyond the saved
// phase.
type PhaseUpdateReport struct {
	// CareBeyondEnd lists the children whose care now runs past the phase's
	// shortened end (#6598). The edit changes none of them: the school ends
	// each care on purpose through the care-end process ("Betreuung
	// beenden"). Empty unless the edit moved the end earlier.
	CareBeyondEnd []CareBeyondPhaseEnd
}

// CareBeyondPhaseEnd is one child confirmed for a phase whose care end lies
// after the phase's new end, while it did not lie after the old one.
type CareBeyondPhaseEnd struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
	// CareEnd is the child's last care day as it stands (enrolled_until).
	CareEnd calendar.Date
	// PhaseEnd is the phase's new last day of care.
	PhaseEnd calendar.Date
}

// PhaseCareStudent is the care state of one child a phase edit reports on.
type PhaseCareStudent struct {
	ID          int64
	FirstName   string
	LastName    string
	SchoolClass string
	// InCare is true while the child's care has not ended: its start is
	// still ahead (pending) or it is cared for (active). A child whose care
	// ended or who graduated is history.
	InCare bool
	// CareEnd is the child's last care day; zero when none is set.
	CareEnd calendar.Date
}

// PhaseCareStudents reads the care state of the given children in the
// caller's transaction. People Directory owns the rows; the edit only reads.
type PhaseCareStudents interface {
	PhaseCareStudents(ctx context.Context, studentIDs []int64) ([]PhaseCareStudent, error)
}
