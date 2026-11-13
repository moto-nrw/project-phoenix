package owners

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	enrollmentOwner "github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	timezone "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// phaseResponseRoster feeds the response overview (#3379) with the children
// the school cares for today; Enrollment decides which of them it asks.
type phaseResponseRoster struct {
	query peopledirectory.StudentRosterQuery
}

func (d phaseResponseRoster) ListRunningStudents(ctx context.Context, today timezone.Date) ([]enrollmentOwner.PhaseResponseStudent, error) {
	entries, err := d.query.ListStudentRoster(ctx, today.String())
	if err != nil {
		return nil, err
	}
	result := make([]enrollmentOwner.PhaseResponseStudent, 0, len(entries))
	for _, entry := range entries {
		result = append(result, enrollmentOwner.PhaseResponseStudent{
			ID: entry.Record.ID, FirstName: entry.FirstName, LastName: entry.LastName,
			SchoolClass: entry.Record.SchoolClass, NotActive: entry.Record.Status != peopledirectory.StudentStatusActive,
		})
	}
	return result, nil
}

type phaseResponseCareExits struct{ query careplan.CareRecordsQuery }

func (d phaseResponseCareExits) StudentsWithCareExit(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	exits, err := d.query.FindCareExits(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(exits))
	for studentID := range exits {
		result[studentID] = true
	}
	return result, nil
}

// phaseCareStudents reads the care end of the children a phase edit
// reports on (#6598) from People Directory: status and care end of the
// student rows, names from the directory's name projection.
type phaseCareStudents struct {
	query interface {
		ListStudentsByID(context.Context, []int64) ([]peopledirectory.Student, error)
		ListStudentNamesByID(context.Context, []int64) ([]peopledirectory.StudentName, error)
	}
}

// NewEnrollmentPhaseCareStudents binds the phase edit's report to People
// Directory.
func NewEnrollmentPhaseCareStudents(query peopledirectory.StudentQuery) enrollmentOwner.PhaseCareStudents {
	return phaseCareStudents{query: query}
}

func (d phaseCareStudents) PhaseCareStudents(ctx context.Context, studentIDs []int64) ([]enrollmentOwner.PhaseCareStudent, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}
	students, err := d.query.ListStudentsByID(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("phase care students: list students: %w", err)
	}
	names, err := d.query.ListStudentNamesByID(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("phase care students: list names: %w", err)
	}
	byID := make(map[int64]peopledirectory.StudentName, len(names))
	for _, name := range names {
		byID[name.StudentID] = name
	}
	result := make([]enrollmentOwner.PhaseCareStudent, 0, len(students))
	for _, student := range students {
		value := enrollmentOwner.PhaseCareStudent{
			ID: student.ID, FirstName: byID[student.ID].FirstName, LastName: byID[student.ID].LastName,
			SchoolClass: student.SchoolClass,
			InCare:      student.Status == peopledirectory.StudentStatusPending || student.Status == peopledirectory.StudentStatusActive,
		}
		if student.EnrolledUntil != "" {
			end, err := timezone.ParseDate(student.EnrolledUntil)
			if err != nil {
				return nil, fmt.Errorf("student %d: care end: %w", student.ID, err)
			}
			value.CareEnd = end
		}
		result = append(result, value)
	}
	return result, nil
}
