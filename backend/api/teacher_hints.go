package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// listTeacherHints reads People Directory's day hints of the Lehrkraft
// (#4795) for many children, oldest first. Each OGS view owns its wire type;
// its binding only translates the rows.
func listTeacherHints(
	ctx context.Context, hints peopledirectory.SchoolDayHintQuery, date calendar.Date, studentIDs []int64,
) ([]peopledirectory.SchoolDayHint, error) {
	if hints == nil {
		return nil, fmt.Errorf("teacher hints: people directory is not configured")
	}
	if len(studentIDs) == 0 {
		return nil, nil
	}
	return hints.ListSchoolDayHints(ctx, date, studentIDs)
}

// newTimetableTeacherHints binds People Directory's day hints (#4795) for the
// block roster; without the directory no hint is attached.
func newTimetableTeacherHints(hints peopledirectory.SchoolDayHintQuery) timetableAPI.TeacherHints {
	if hints == nil {
		return nil
	}
	return timetableTeacherHints{hints: hints}
}

type timetableTeacherHints struct {
	hints peopledirectory.SchoolDayHintQuery
}

func (b timetableTeacherHints) TeacherHints(ctx context.Context, studentIDs []int64, day calendar.Date) (map[int64][]timetable.OperationTeacherHint, error) {
	rows, err := listTeacherHints(ctx, b.hints, day, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]timetable.OperationTeacherHint)
	for _, hint := range rows {
		result[hint.StudentID] = append(result[hint.StudentID], timetable.OperationTeacherHint{
			Body: hint.Body, AuthorName: hint.AuthorName, SchoolClass: hint.SchoolClass,
		})
	}
	return result, nil
}
