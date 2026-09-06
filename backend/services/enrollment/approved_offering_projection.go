package enrollment

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	legacy "github.com/moto-nrw/project-phoenix/models/enrollment"
	owner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type ApprovedOfferingReader interface {
	ListApprovedChildrenByCareOfferingIDs(context.Context, []int64, timezone.Date) ([]*legacy.ApprovedOfferingChild, error)
}

type ApprovedSelectionReader interface {
	ApprovedSelectionsForStudents(context.Context, []int64, owner.Date, owner.Date) ([]*owner.ApprovedOfferingSelection, error)
	ApprovedSelectionsForOfferings(context.Context, []int64, owner.Date) ([]*owner.ApprovedOfferingSelection, error)
}

type OfferingStudent struct {
	ID          int64
	SchoolClass string
	Alumnus     bool
}

type OfferingStudentDirectory interface {
	ListOfferingStudents(context.Context, []int64) ([]OfferingStudent, error)
}

// ApprovedOfferingProjection combines Enrollment references with People
// Directory's current status and class, preserving selection order.
type ApprovedOfferingProjection struct {
	selections ApprovedSelectionReader
	students   OfferingStudentDirectory
}

func NewApprovedOfferingProjection(selections ApprovedSelectionReader, students OfferingStudentDirectory) *ApprovedOfferingProjection {
	return &ApprovedOfferingProjection{selections: selections, students: students}
}

func (p *ApprovedOfferingProjection) ListApprovedChildrenByCareOfferingIDs(ctx context.Context, offeringIDs []int64, onOrAfter timezone.Date) ([]*legacy.ApprovedOfferingChild, error) {
	result := make([]*legacy.ApprovedOfferingChild, 0)
	if len(offeringIDs) == 0 {
		return result, nil
	}
	selections, err := p.selections.ApprovedSelectionsForOfferings(ctx, offeringIDs, owner.Date(onOrAfter))
	if err != nil {
		return nil, err
	}
	return p.resolveSelections(ctx, selections)
}

func (p *ApprovedOfferingProjection) resolveSelections(ctx context.Context, selections []*owner.ApprovedOfferingSelection) ([]*legacy.ApprovedOfferingChild, error) {
	result := make([]*legacy.ApprovedOfferingChild, 0)
	if len(selections) == 0 {
		return result, nil
	}
	if p.students == nil {
		return nil, errors.New("enrollment repositories: student directory is not bound")
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
		result = append(result, &legacy.ApprovedOfferingChild{
			Link:      legacyOfferingSelections([]*owner.RequestChildOffering{selection.Selection})[0],
			StudentID: student.ID, SchoolClass: student.SchoolClass,
		})
	}
	return result, nil
}

func (p *ApprovedOfferingProjection) ListApprovedByStudentIDsInRange(ctx context.Context, ids []int64, from, to timezone.Date) ([]*legacy.ApprovedOfferingChild, error) {
	result := make([]*legacy.ApprovedOfferingChild, 0)
	if len(ids) == 0 || to.Before(from) {
		return result, nil
	}
	if p.students == nil {
		return nil, errors.New("enrollment repositories: student directory is not bound")
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
	selections, err := p.selections.ApprovedSelectionsForStudents(ctx, enrolled, owner.Date(from), owner.Date(to))
	if err != nil {
		return nil, err
	}
	return p.resolveSelections(ctx, selections)
}
