package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// beyondEndOwner serves the confirmed children of one phase and their
// approvals in other phases; every other owner call would panic on the nil
// embedded interface, which the report must never reach.
type beyondEndOwner struct {
	PhaseRecords
	children []enrollment.PhaseResponseChild
	periods  []*enrollment.StudentCarePeriod
}

func (o beyondEndOwner) PhaseResponseChildren(_ context.Context, _ int64, statuses []string) ([]enrollment.PhaseResponseChild, error) {
	if len(statuses) != 1 || statuses[0] != enrollment.ChildStatusApproved {
		return nil, nil
	}
	return o.children, nil
}

func (o beyondEndOwner) StudentCarePeriodsByStudentIDs(context.Context, []int64) ([]*enrollment.StudentCarePeriod, error) {
	return o.periods, nil
}

type beyondEndStudents []enrollment.PhaseCareStudent

func (s beyondEndStudents) PhaseCareStudents(_ context.Context, ids []int64) ([]enrollment.PhaseCareStudent, error) {
	wanted := map[int64]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	var result []enrollment.PhaseCareStudent
	for _, student := range s {
		if wanted[student.ID] {
			result = append(result, student)
		}
	}
	return result, nil
}

func confirmedChild(childID, studentID int64) enrollment.PhaseResponseChild {
	return enrollment.PhaseResponseChild{IsPhaseChild: true, Child: &enrollment.RequestChild{ID: childID, CreatedStudentID: &studentID}}
}

// A phase shortened from 31.07. to 30.06. names exactly the children whose
// care this edit cut off (#6598): pending or active, a care end after the
// new and not after the old end, not yet passed, and not reached by an
// approval in another phase. It sorts them by name in German order.
func TestCareBeyondShortenedEnd_ListsTheChildrenTheEditCutOff(t *testing.T) {
	t.Parallel()

	const phaseID, otherPhaseID = 40, 41
	oldEnd, newEnd, today := calendar.Date("2028-07-31"), calendar.Date("2028-06-30"), calendar.Date("2028-05-02")
	source := func(id int64) *int64 { return &id }
	children := []enrollment.PhaseResponseChild{
		confirmedChild(501, 601), // active, phase end: listed
		confirmedChild(502, 602), // pending, end inside the cut: listed
		confirmedChild(503, 603), // already ends on the new end
		confirmedChild(504, 604), // end later than the old phase end: set by hand
		confirmedChild(505, 605), // care ended (inactive)
		confirmedChild(506, 606), // graduated (alumnus)
		confirmedChild(507, 607), // another phase reaches the end
		confirmedChild(508, 608), // another phase ends earlier: listed
		confirmedChild(509, 609), // no care end
		{IsPhaseChild: false, Child: &enrollment.RequestChild{ID: 510, CreatedStudentID: source(610)}}, // rollover source
		{IsPhaseChild: true, Child: &enrollment.RequestChild{ID: 511}},                                 // no child record
	}
	students := beyondEndStudents{
		{ID: 601, FirstName: "Mia", LastName: "Lang", SchoolClass: "2b", InCare: true, CareEnd: oldEnd},
		{ID: 602, FirstName: "Ali", LastName: "Ärmel", SchoolClass: "1a", InCare: true, CareEnd: "2028-07-10"},
		{ID: 603, InCare: true, CareEnd: newEnd},
		{ID: 604, InCare: true, CareEnd: oldEnd.AddDays(1)},
		{ID: 605, CareEnd: oldEnd},
		{ID: 606, CareEnd: oldEnd},
		{ID: 607, InCare: true, CareEnd: oldEnd},
		{ID: 608, FirstName: "Ben", LastName: "Lang", SchoolClass: "3c", InCare: true, CareEnd: oldEnd},
		{ID: 609, InCare: true},
		{ID: 610, InCare: true, CareEnd: oldEnd},
	}
	periods := []*enrollment.StudentCarePeriod{
		{StudentID: 601, PhaseID: phaseID, ServiceEndDate: enrollment.Date(newEnd)},
		{StudentID: 607, PhaseID: otherPhaseID, ServiceEndDate: enrollment.Date(oldEnd)},
		{StudentID: 608, PhaseID: otherPhaseID, ServiceEndDate: "2028-07-15"},
	}
	s := newPhases(PhaseDependencies{
		Records:      beyondEndOwner{children: children, periods: periods},
		CareStudents: students,
		Today:        func() calendar.Date { return today },
	})
	before := &enrollment.Phase{ID: phaseID, ServiceStartDate: "2027-08-01", ServiceEndDate: enrollment.Date(oldEnd)}
	after := &enrollment.Phase{ID: phaseID, ServiceStartDate: "2027-08-01", ServiceEndDate: enrollment.Date(newEnd)}

	got, err := s.careBeyondShortenedEnd(context.Background(), before, after)
	require.NoError(t, err)
	assert.Equal(t, []enrollment.CareBeyondPhaseEnd{
		{StudentID: 602, FirstName: "Ali", LastName: "Ärmel", SchoolClass: "1a", CareEnd: "2028-07-10", PhaseEnd: newEnd},
		{StudentID: 608, FirstName: "Ben", LastName: "Lang", SchoolClass: "3c", CareEnd: oldEnd, PhaseEnd: newEnd},
		{StudentID: 601, FirstName: "Mia", LastName: "Lang", SchoolClass: "2b", CareEnd: oldEnd, PhaseEnd: newEnd},
	}, got)

	// A care end that has passed is history, even when the edit cut it.
	s.deps.Today = func() calendar.Date { return "2028-07-11" }
	got, err = s.careBeyondShortenedEnd(context.Background(), before, after)
	require.NoError(t, err)
	assert.Equal(t, []int64{608, 601}, beyondEndIDs(got), "the child whose care ended on 10.07. is history")

	// Only a shortened end reports: a longer end, the same end and a moved
	// start leave every care end inside the phase.
	for _, end := range []calendar.Date{oldEnd, oldEnd.AddDays(14)} {
		moved := *after
		moved.ServiceEndDate = enrollment.Date(end)
		moved.ServiceStartDate = "2027-09-01"
		got, err = s.careBeyondShortenedEnd(context.Background(), before, &moved)
		require.NoError(t, err)
		assert.Empty(t, got, "end %s", end)
	}
}

func beyondEndIDs(children []enrollment.CareBeyondPhaseEnd) []int64 {
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.StudentID)
	}
	return ids
}
