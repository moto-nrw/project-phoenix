package services

import (
	"context"
	"errors"
	"fmt"

	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The staff, teacher, guest and teaching-assignment adapters report their
// failures in the retained repository error shape (models/users), so callers
// keep classifying with errors.Is(err, sql.ErrNoRows) and
// userModels.IsRepositoryNoRows(err) alike. They moved here from the
// repository composition with the adapters (#2743).

// repositoryNotFound builds the not-found shape every retained repository
// returns.
func repositoryNotFound(op string) error {
	return &userModels.DatabaseError{Op: op, Err: errors.Join(userModels.ErrRepositoryNotFound, transactionruntime.ErrNoRows)}
}

// repositoryError wraps err in the retained repository error shape,
// preserving the chain so errors.Is on the original sentinel still works.
func repositoryError(op string, err error) error {
	if err == nil {
		return nil
	}
	return &userModels.DatabaseError{Op: op, Err: err}
}

// calendarDateString renders an optional calendar date as YYYY-MM-DD, empty
// when unset.
func calendarDateString(value *calendar.Date) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.String()
}

// parseCalendarDate parses a YYYY-MM-DD value; an empty or malformed value
// yields nil, which is how the retained models spell "no date".
func parseCalendarDate(value string) *calendar.Date {
	if value == "" {
		return nil
	}
	parsed, err := calendar.ParseDate(value)
	if err != nil {
		return nil
	}
	return &parsed
}

// staffLookup is the slice of the School Membership query the projections
// read: who a staff ID is, and which teacher profile belongs to it.
type staffLookup interface {
	ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error)
	ListTeachers(context.Context, schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, error)
}

// staffByID resolves the staff members for ids through the owner query,
// keyed by staff ID. includeDeleted mirrors whether the replaced SQL join
// filtered soft-deleted rows.
func staffByID(ctx context.Context, query staffLookup, ids []int64, includeDeleted bool) (map[int64]schoolmembership.Staff, error) {
	unique := uniqueIDs(ids)
	if len(unique) == 0 {
		return map[int64]schoolmembership.Staff{}, nil
	}
	values, err := query.ListStaff(ctx, schoolmembership.StaffFilter{IDs: unique, IncludeDeleted: includeDeleted})
	if err != nil {
		return nil, fmt.Errorf("load staff: %w", err)
	}
	result := make(map[int64]schoolmembership.Staff, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result, nil
}

func uniqueIDs(ids []int64) []int64 {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, found := seen[id]; found {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

// membershipEmploymentError restates Workforce's employment refusals in the
// School Membership vocabulary the staff adapters map.
func membershipEmploymentError(err error) error {
	switch {
	case errors.Is(err, workforce.ErrPersonnelNumberTaken):
		return schoolmembership.ErrPersonnelNumberConflict
	case errors.Is(err, workforce.ErrStaffEmploymentNotFound):
		return schoolmembership.ErrStaffNotFound
	}
	return err
}
