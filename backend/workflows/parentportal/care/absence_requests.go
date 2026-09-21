package care

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
)

// ListExcusedRequests is the legacy-named read path for the child's pending and
// recently decided absence requests submitted by the calling guardian. The
// child's effective absence state is shared separately; request notes and
// decision reasons remain private to their submitter.
func (s *Service) ListExcusedRequests(ctx context.Context, accountID, studentID int64) ([]*careplan.ExcusedAbsenceRequest, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if s.ExcusedRequests == nil {
		return []*careplan.ExcusedAbsenceRequest{}, nil
	}
	// Show rejected/withdrawn requests for two weeks so a parent learns the
	// outcome, while pending ones show regardless of age.
	recentSince := time.Now().AddDate(0, 0, -14)
	var out []*careplan.ExcusedAbsenceRequest
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		rows, err := s.ExcusedRequests.ListForStudent(txCtx, studentID, recentSince)
		if err != nil {
			return err
		}
		visibility, visibilityErr := s.RequestSharing.LoadRequestShareVisibility(txCtx, studentID)
		if visibilityErr != nil {
			return visibilityErr
		}
		out = visibleExcusedRequests(rows, accountID, visibility)
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list excused requests: %w", txErr)
	}
	return out, nil
}

// carePlanDates hands the calendar days to the Care Plan owner in its own
// canonical form; both types carry YYYY-MM-DD.
func carePlanDates(dates []timezone.Date) []careplan.Date {
	result := make([]careplan.Date, len(dates))
	for i := range dates {
		result[i] = careplan.Date(dates[i])
	}
	return result
}

func visibleExcusedRequests(
	rows []*careplan.ExcusedAbsenceRequest, accountID int64, visibility RequestShareVisibility,
) []*careplan.ExcusedAbsenceRequest {
	out := make([]*careplan.ExcusedAbsenceRequest, 0, len(rows))
	for _, row := range rows {
		if row != nil && visibility.Allows(RequestShareExcused, row.ID, accountID, row.SubmittedBy) {
			out = append(out, row)
		}
	}
	return out
}

// mapExcusedRequestError translates the absence domain's sentinels into the
// parent package's, so handlers keep switching on one set. Everything else is
// wrapped, which keeps errors.Is working for the shared request sentinels
// (stale, reason required).
func mapExcusedRequestError(err error, op string) error {
	switch {
	case errors.Is(err, careplan.ErrExcusedRequestNoDates):
		return ErrNoDates
	case errors.Is(err, careplan.ErrExcusedRequestEmptyNote):
		return ErrEmptyNote
	case errors.Is(err, careplan.ErrExcusedRequestNoteTooLong):
		return ErrNoteTooLong
	case errors.Is(err, careplan.ErrExcusedRequestOverlap):
		return ErrExcusedRequestOverlap
	// A planned partial-day excusal already owns one of the requested dates
	// (same refusal as a direct parent status write that Care Plan refuses
	// with ErrManualPartialAbsenceConflict). Surface as the existing care
	// conflict so the handler returns HTTP 409, not 500.
	case errors.Is(err, careplan.ErrExcusedRequestStatusConflict):
		return ErrCareExceptionConflict
	case errors.Is(err, careplan.ErrExcusedRequestNotFound):
		return ErrExcusedRequestNotFound
	case errors.Is(err, careplan.ErrExcusedRequestNotPending):
		return ErrExcusedRequestNotPending
	default:
		return fmt.Errorf("parent: %s: %w", op, err)
	}
}

// EditExcusedRequest rewrites the caller's own pending absence request
// (#2267, story 37). It replaces withdrawal: a guardian who picked the wrong
// day corrects it, the request keeps its id and the co-guardians it was shared
// with stay recipients. Same gate as the withdraw it replaces — available
// while the child's care is running, ownership enforced in the absence
// service.
func (s *Service) EditExcusedRequest(
	ctx context.Context, accountID, studentID, requestID int64,
	dates []timezone.Date, note, expectedVersion string,
) (*careplan.ExcusedAbsenceRequest, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if s.ExcusedRequests == nil {
		return nil, ErrExcusedRequestNotFound
	}
	var out *careplan.ExcusedAbsenceRequest
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if student.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		req, editErr := s.ExcusedRequests.EditRequest(txCtx, careplan.ExcusedRequestEditInput{
			RequestID:         requestID,
			StudentID:         studentID,
			GuardianAccountID: accountID,
			ExpectedVersion:   expectedVersion,
			Dates:             carePlanDates(dates),
			Note:              note,
			NoteRequired:      s.guardianReasonRequired(ctx, child.TenantID),
		})
		if editErr != nil {
			return editErr
		}
		out = req
		return nil
	})
	if txErr != nil {
		return nil, mapExcusedRequestError(txErr, "edit excused request")
	}
	return out, nil
}

// ListSickDays returns the child's active parent-facing absences in [from, to]:
// sick ("Krankmeldung") days plus the parent's own excused ("Termin/Abwesenheit")
// days, so a parent sees every absence they reported. Staff-created excused days
// (source=planned/manual) are an internal scheduled status the parent neither set
// nor manages here, so they are NOT surfaced. Class-trip days stay excluded for
// the same reason.
func (s *Service) ListSickDays(ctx context.Context, accountID, studentID int64, from, to timezone.Date) ([]*absencerecords.StudentStatusDay, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}

	var out []*absencerecords.StudentStatusDay
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		rows, err := s.StatusDayRepo.FindActiveByStudentAndDateRange(txCtx, studentID, from, to)
		if err != nil {
			return err
		}
		absences := make([]*absencerecords.StudentStatusDay, 0, len(rows))
		for _, r := range rows {
			switch {
			case r.Status == absencerecords.StudentStatusDaySick:
				absences = append(absences, parentVisibleStatusDay(r, accountID))
			case r.Status == absencerecords.StudentStatusDayExcused &&
				r.Source == absencerecords.StudentStatusSourceParent:
				// Only parent-reported excused days belong in the parents
				// portal; staff-created excused rows (planned/manual) stay
				// internal so we don't leak their note/source to guardians.
				absences = append(absences, parentVisibleStatusDay(r, accountID))
			}
		}
		out = absences
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list absences: %w", txErr)
	}
	return out, nil
}

func parentVisibleStatusDay(row *absencerecords.StudentStatusDay, accountID int64) *absencerecords.StudentStatusDay {
	visible := *row
	if row.GuardianAccountID == nil || *row.GuardianAccountID != accountID {
		visible.Note = nil
	}
	return &visible
}
