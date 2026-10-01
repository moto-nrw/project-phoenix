package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var errOfferingStudentsUnbound = errors.New("enrollment repositories: student directory is not bound")

// ApprovedOfferingProjection combines Enrollment's approved selections with
// People Directory's current status and class, preserving selection order.
type ApprovedOfferingProjection struct {
	selections ApprovedSelections
	students   OfferingStudentDirectory
}

// NewApprovedOfferingProjection binds the projection to its read ports.
func NewApprovedOfferingProjection(selections ApprovedSelections, students OfferingStudentDirectory) *ApprovedOfferingProjection {
	return &ApprovedOfferingProjection{selections: selections, students: students}
}

// ListApprovedChildrenByCareOfferingIDs lists the approved, still-enrolled
// children of the given offerings whose selection is effective on or after
// the given day.
func (p *ApprovedOfferingProjection) ListApprovedChildrenByCareOfferingIDs(ctx context.Context, offeringIDs []int64, onOrAfter calendar.Date) ([]*enrollment.ApprovedOfferingChild, error) {
	result := make([]*enrollment.ApprovedOfferingChild, 0)
	if len(offeringIDs) == 0 {
		return result, nil
	}
	selections, err := p.selections.ApprovedSelectionsForOfferings(ctx, offeringIDs, enrollment.Date(onOrAfter))
	if err != nil {
		return nil, err
	}
	return p.resolveSelections(ctx, selections)
}

func (p *ApprovedOfferingProjection) resolveSelections(ctx context.Context, selections []*enrollment.ApprovedOfferingSelection) ([]*enrollment.ApprovedOfferingChild, error) {
	result := make([]*enrollment.ApprovedOfferingChild, 0)
	if len(selections) == 0 {
		return result, nil
	}
	if p.students == nil {
		return nil, errOfferingStudentsUnbound
	}
	ids := make([]int64, 0, len(selections))
	seen := make(map[int64]bool)
	for _, selection := range selections {
		if !seen[selection.StudentID] {
			ids = append(ids, selection.StudentID)
			seen[selection.StudentID] = true
		}
	}
	students, err := p.students.ListOfferingStudents(ctx, ids)
	if err != nil {
		return nil, err
	}
	enrolled := make(map[int64]OfferingStudent, len(students))
	for _, student := range students {
		if !student.Alumnus {
			enrolled[student.ID] = student
		}
	}
	for _, selection := range selections {
		student, ok := enrolled[selection.StudentID]
		if !ok {
			continue
		}
		result = append(result, &enrollment.ApprovedOfferingChild{
			Link:      enrollment.RequestChildOfferingRecordsOf([]*enrollment.RequestChildOffering{selection.Selection})[0],
			StudentID: student.ID, SchoolClass: student.SchoolClass,
		})
	}
	return result, nil
}

// ListApprovedByStudentIDsInRange lists the approved bookings of the given
// still-enrolled children that overlap the given days, for Care Plan's
// baselines.
func (p *ApprovedOfferingProjection) ListApprovedByStudentIDsInRange(ctx context.Context, ids []int64, from, to calendar.Date) ([]*careplan.ApprovedBooking, error) {
	result := make([]*careplan.ApprovedBooking, 0)
	if len(ids) == 0 || to.Before(from) {
		return result, nil
	}
	if p.students == nil {
		return nil, errOfferingStudentsUnbound
	}
	students, err := p.students.ListOfferingStudents(ctx, ids)
	if err != nil {
		return nil, err
	}
	enrolled := make([]int64, 0, len(students))
	for _, student := range students {
		if !student.Alumnus {
			enrolled = append(enrolled, student.ID)
		}
	}
	if len(enrolled) == 0 {
		return result, nil
	}
	selections, err := p.selections.ApprovedSelectionsForStudents(ctx, enrolled, enrollment.Date(from), enrollment.Date(to))
	if err != nil {
		return nil, err
	}
	children, err := p.resolveSelections(ctx, selections)
	if err != nil {
		return nil, err
	}
	for _, child := range children {
		booking := &careplan.ApprovedBooking{StudentID: child.StudentID}
		if child.Link != nil {
			booking.Link = &careplan.BookingSelection{
				CareOfferingID: child.Link.CareOfferingID,
				SelectedDays:   child.Link.SelectedDays,
				ValidFrom:      child.Link.ValidFrom,
				ValidUntil:     child.Link.ValidUntil,
			}
		}
		result = append(result, booking)
	}
	return result, nil
}
