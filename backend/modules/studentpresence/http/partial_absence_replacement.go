package studentpresencehttp

import (
	context "context"
	errors "errors"
	"fmt"
	http "net/http"
	slices "slices"

	render "github.com/go-chi/render"
	common "github.com/moto-ogs/project-phoenix/api/common"
	absencerecords "github.com/moto-ogs/project-phoenix/modules/careplan/absencerecords"
	peopleModule "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	calendar "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// PartialAbsenceReplacer is Student Presence's port onto Care Plan's command
// that removes staff-entered partial absences so a full-day status can take
// their place (#5698). The root binds it; it joins the status write's unit.
type PartialAbsenceReplacer interface {
	ReplaceManualPartialAbsences(ctx context.Context, studentID int64, dates []calendar.Date) error
}

// partialAbsenceReplacement validates a confirmed replacement and returns the
// hook the status write runs under its locks. No confirmed day returns nil:
// every manual partial absence then refuses the write as before.
//
// Removing a partial absence is the same write as deleting it, so the caller
// also needs the partial-absence write gate: full write access to the child
// and the "excused" status gate. An absence-only role does not have it (#2232).
func (rs *Resource) partialAbsenceReplacement(
	r *http.Request, userPermissions []string, student *peopleModule.StudentState, rawDates []string, requested []calendar.Date,
) (func(context.Context, int64) error, render.Renderer) {
	ctx := r.Context()
	if len(rawDates) == 0 {
		return nil, nil
	}
	dates, err := parseStatusDayDates(rawDates)
	if err != nil {
		return nil, common.ErrorInvalidRequest(err)
	}
	for _, date := range dates {
		if !slices.Contains(requested, date) {
			return nil, common.ErrorInvalidRequest(fmt.Errorf("replace_partial_absence_dates: %s is not a requested day", date))
		}
	}
	if rs.PartialAbsences == nil {
		return nil, common.ErrorInternalServer(errors.New("partial absence replacement is not configured"))
	}
	if !rs.checkStudentFullAccess(r, student) {
		return nil, common.ErrorForbidden(errors.New("full access required to replace partial absences"))
	}
	if ok, authErr := rs.canManageStudentStatus(ctx, userPermissions, student, absencerecords.StudentStatusDayExcused); !ok {
		if authErr == nil {
			authErr = errors.New("partial absence write permission required")
		}
		return nil, common.ErrorForbidden(authErr)
	}
	replacer := rs.PartialAbsences
	return func(ctx context.Context, studentID int64) error {
		return replacer.ReplaceManualPartialAbsences(ctx, studentID, dates)
	}, nil
}
