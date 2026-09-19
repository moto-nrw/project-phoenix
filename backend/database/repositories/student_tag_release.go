package repositories

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// StudentTagReleaser frees the bracelets of students through the People
// Directory: the person rows belong to that owner, the student rows only
// supply the person ids. The care lifecycle uses it when a child's care ends
// (#2711 moved the grade-transition release into its workflow).
type StudentTagReleaser struct{ people peopledirectory.Capability }

func NewStudentTagReleaser(people peopledirectory.Capability) StudentTagReleaser {
	return StudentTagReleaser{people: people}
}

// ReleaseStudentTagsByIDs clears the RFID tag on the given students' person
// rows and returns what each of them was holding, keyed by student id.
func (r StudentTagReleaser) ReleaseStudentTagsByIDs(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}
	students, err := r.people.ListStudentsByID(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	personIDs := make([]int64, 0, len(students))
	studentsByPerson := make(map[int64]int64, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
		studentsByPerson[student.PersonID] = student.ID
	}
	released, err := r.people.ReleaseTags(ctx, personIDs)
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

// StudentTagReleaser exposes the bound People Directory's bracelet release
// to the care lifecycle; it adds no repository to the legacy graph.
func (f *Factory) StudentTagReleaser() StudentTagReleaser {
	return NewStudentTagReleaser(f.students)
}
