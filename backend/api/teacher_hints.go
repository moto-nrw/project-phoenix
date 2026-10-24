package api

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// teacherHintsByStudent reads People Directory's day hints of the Lehrkraft
// (#4795) for many children and groups them by child, oldest first, in the
// projection's own shape. Each OGS view owns its wire type; the root only
// translates.
func teacherHintsByStudent[T any](
	ctx context.Context, hints peopledirectory.SchoolDayHintQuery, date calendar.Date, studentIDs []int64,
	convert func(peopledirectory.SchoolDayHint) T,
) (map[int64][]T, error) {
	result := make(map[int64][]T)
	if hints == nil {
		return nil, fmt.Errorf("teacher hints: people directory is not configured")
	}
	if len(studentIDs) == 0 {
		return result, nil
	}
	rows, err := hints.ListSchoolDayHints(ctx, date, studentIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.StudentID] = append(result[row.StudentID], convert(row))
	}
	return result, nil
}
