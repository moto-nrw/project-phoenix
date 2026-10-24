package application

import (
	"context"
)

func (s *Service) ReleaseStudentTagsByIDs(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}
	students, err := s.ListStudentsByID(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	personIDs := make([]int64, 0, len(students))
	studentsByPerson := make(map[int64]int64, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
		studentsByPerson[student.PersonID] = student.ID
	}
	released, err := s.ReleaseTags(ctx, personIDs)
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
