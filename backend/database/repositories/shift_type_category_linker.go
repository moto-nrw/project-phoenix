package repositories

import (
	"context"
	"errors"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// ShiftTypeCategoryLinker binds the Kategorie↔Schichtart mapping write
// (#1837) the Dienstplan's shift-type administration runs after a write. The
// retained activity service reports unknown or foreign category ids with its
// own sentinel; the Dienstplan classifies on the Timetable owner's (#3424), so
// that failure keeps its text and gains the owner's identity.
func ShiftTypeCategoryLinker(link func(context.Context, int64, []int64) error) func(context.Context, int64, []int64) error {
	return func(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error {
		err := link(ctx, shiftTypeID, categoryIDs)
		if errors.Is(err, activitiesModels.ErrUnknownCategoryIDs) {
			return unknownShiftTypeCategoriesError{err: err}
		}
		return err
	}
}

// unknownShiftTypeCategoriesError is a rejected category mapping: it reads
// as the retained error and matches both sentinels.
type unknownShiftTypeCategoriesError struct{ err error }

func (e unknownShiftTypeCategoriesError) Error() string { return e.err.Error() }

func (e unknownShiftTypeCategoriesError) Unwrap() []error {
	return []error{e.err, timetable.ErrUnknownCategoryIDs}
}
