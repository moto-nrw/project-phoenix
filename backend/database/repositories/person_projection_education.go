package repositories

import (
	"context"
	"slices"

	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// personGradeTransitionRepository attaches the student names to the class
// cohort and serves the RFID tag commands through the People Directory.
type personGradeTransitionRepository struct {
	educationModels.GradeTransitionRepository
	persons peopledirectory.Capability
}

func (r personGradeTransitionRepository) GetStudentsByClasses(ctx context.Context, classes []string) ([]*educationModels.StudentClassInfo, error) {
	rows, err := r.GradeTransitionRepository.GetStudentsByClasses(ctx, classes)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.PersonID)
	}
	persons, err := personsByID(ctx, r.persons, ids)
	if err != nil {
		return nil, err
	}
	// The cohort keeps every student: a person the directory no longer
	// shows (soft-deleted) stays in the transition with an empty name and
	// sorts last within its class.
	for _, row := range rows {
		if person, found := persons[row.PersonID]; found {
			row.PersonName = person.FullName()
		}
	}
	slices.SortStableFunc(rows, func(left, right *educationModels.StudentClassInfo) int {
		if order := compareStrings(left.SchoolClass, right.SchoolClass); order != 0 {
			return order
		}
		leftPerson, leftFound := persons[left.PersonID]
		rightPerson, rightFound := persons[right.PersonID]
		if leftFound != rightFound {
			if leftFound {
				return -1
			}
			return 1
		}
		if order := compareStrings(leftPerson.LastName, rightPerson.LastName); order != 0 {
			return order
		}
		return compareStrings(leftPerson.FirstName, rightPerson.FirstName)
	})
	return rows, nil
}

// ReleaseStudentTagsByIDs locks the students' persons through the owner
// command, clears their tags, and reports what each student was holding.
func (r personGradeTransitionRepository) ReleaseStudentTagsByIDs(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}
	personIDs, err := r.PersonIDsByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(personIDs))
	studentsByPerson := make(map[int64]int64, len(personIDs))
	for studentID, personID := range personIDs {
		ids = append(ids, personID)
		studentsByPerson[personID] = studentID
	}
	released, err := r.persons.ReleaseTags(ctx, ids)
	if err != nil {
		return nil, err
	}
	if len(released) == 0 {
		return nil, nil
	}
	result := make(map[int64]string, len(released))
	for _, entry := range released {
		result[studentsByPerson[entry.PersonID]] = entry.TagID
	}
	return result, nil
}

// RestoreStudentTag re-links a released tag through the owner command; a
// student without a person row, or whose tag found a new holder, reports
// false without failing the revert.
func (r personGradeTransitionRepository) RestoreStudentTag(ctx context.Context, studentID int64, tagID string) (bool, error) {
	if tagID == "" {
		return false, nil
	}
	personIDs, err := r.PersonIDsByStudentIDs(ctx, []int64{studentID})
	if err != nil {
		return false, err
	}
	personID, found := personIDs[studentID]
	if !found {
		return false, nil
	}
	return r.persons.RestoreTag(ctx, personID, tagID)
}
