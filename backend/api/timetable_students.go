package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

func timetableStudents(students peopledirectory.StudentQuery) timetableCompose.StudentDirectory {
	return timetableCompose.StudentDirectoryFunc(func(ctx context.Context) ([]timetableCompose.TargetStudent, error) {
		values, err := students.ListEnrolledStudents(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]timetableCompose.TargetStudent, 0, len(values))
		for _, value := range values {
			result = append(result, timetableCompose.TargetStudent{
				ID: value.ID, SchoolClass: value.SchoolClass, EducationGroupID: value.GroupID,
				EnrolledUntil: value.EnrolledUntil,
			})
		}
		return result, nil
	})
}
