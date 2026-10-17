package care

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// EditPickupChangeRequest rewrites the caller's own pending one-day pickup
// change (#2267, story 37).
func (s *Service) EditPickupChangeRequest(
	ctx context.Context, accountID, studentID, requestID int64,
	date calendar.Date, pickupTime time.Time, departureMode, reason, expectedVersion string,
) (*carerequests.Request, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, GuardianPermissionPickupManage)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if err := s.checkDayDepartureModeAllowed(ctx, child.TenantID, date, departureMode); err != nil {
		return nil, err
	}
	if s.CareRequests == nil {
		return nil, errors.New("parent: pickup change request service not configured")
	}
	var out *carerequests.Request
	txErr := s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if student.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		cutoff, err := s.pickupChangeCutoffInTx(txCtx, child.TenantID)
		if err != nil {
			return err
		}
		req, editErr := s.CareRequests.EditRequest(txCtx, carerequests.EditInput{
			RequestID:         requestID,
			StudentID:         studentID,
			GuardianAccountID: accountID,
			ExpectedVersion:   expectedVersion,
			Date:              date,
			PickupTime:        pickupTime,
			DepartureMode:     departureMode,
			Reason:            reason,
			// The reason is mandatory only while the school asks the family
			// for one (#2267, story 28).
			ReasonRequired: s.guardianReasonRequired(ctx, child.TenantID),
			Cutoff:         cutoff,
		})
		if editErr != nil {
			return editErr
		}
		out = req
		return nil
	})
	if txErr != nil {
		return nil, MapCareRequestError(txErr, "edit pickup change request")
	}
	return out, nil
}

// SubmitPickupChangeRequest asks the OGS for a different Gehzeit, a different
// Abholart (#3945), or both, on one day. A zero pickupTime or an empty
// departureMode leaves that leg of the day unchanged.
func (s *Service) SubmitPickupChangeRequest(ctx context.Context, accountID, studentID int64, date calendar.Date, pickupTime time.Time, departureMode, reason string, recipientGuardianProfileIDs []int64) (*carerequests.Request, error) {
	reason = strings.TrimSpace(reason)
	if err := validatePickupChangeShape(pickupTime, departureMode, reason); err != nil {
		return nil, err
	}
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, GuardianPermissionPickupManage)
	if err != nil {
		return nil, err
	}
	reasonRequired, today, err := s.checkPickupChangeSubmittable(ctx, child, reason, date)
	if err != nil {
		return nil, err
	}
	if err := s.checkDayDepartureModeAllowed(ctx, child.TenantID, date, departureMode); err != nil {
		return nil, err
	}
	legs := pickupChangeLegs{pickupTime: !pickupTime.IsZero(), departureMode: departureMode != ""}
	var result *carerequests.Request
	err = s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		if err := s.lockPickupChangeDay(txCtx, studentID, date, today, legs); err != nil {
			return err
		}
		policy, err := s.pickupChangePolicyInTx(txCtx, child.TenantID)
		if err != nil {
			return err
		}
		if !policy.enabled {
			return ErrPickupChangeDisabled
		}
		created, createErr := s.CareRequests.CreatePickupChange(txCtx, carerequests.PickupChangeCreateInput{
			StudentID:         studentID,
			GuardianAccountID: accountID,
			Date:              date,
			PickupTime:        pickupTime,
			DepartureMode:     departureMode,
			Reason:            reason,
			// The reason is mandatory only while the school asks the family
			// for one (#2267, story 28).
			ReasonRequired: reasonRequired,
			Cutoff:         policy.cutoff,
		})
		if createErr != nil {
			return createErr
		}
		// Same transaction as the request row, so a refused share never leaves
		// a request the family cannot see (#2267).
		if shareErr := s.RequestSharing.ShareRequestInTx(
			txCtx, accountID, studentID, RequestSharePickupChange, created.ID, recipientGuardianProfileIDs,
		); shareErr != nil {
			return shareErr
		}
		result = created
		return nil
	})
	if err != nil {
		return nil, MapCareRequestError(err, "submit pickup change request")
	}
	return result, nil
}

// validatePickupChangeShape runs the cheap input checks that need no tenant.
func validatePickupChangeShape(pickupTime time.Time, departureMode, reason string) error {
	if pickupTime.IsZero() && departureMode == "" {
		return ErrNoCareException
	}
	if departureMode != "" && !carerequests.RequestableDepartureMode(departureMode) {
		return ErrInvalidCareRequestPayload
	}
	if utf8.RuneCountInString(reason) > 255 {
		return ErrCareExceptionReasonTooLong
	}
	return nil
}

// checkPickupChangeSubmittable runs the tenant-dependent checks that precede
// the write transaction. It returns whether the school requires a reason and
// today's date.
func (s *Service) checkPickupChangeSubmittable(ctx context.Context, child *Child, reason string, date calendar.Date) (bool, calendar.Date, error) {
	// Whether the reason is mandatory is a per-school setting, so it can only
	// be decided once the child — and with it the tenant — is known (#2267,
	// story 28). The cheap shape checks in SubmitPickupChangeRequest still run
	// first.
	reasonRequired := s.guardianReasonRequired(ctx, child.TenantID)
	if reason == "" && reasonRequired {
		return false, "", ErrCareExceptionReasonRequired
	}
	// A child whose care at this school has ended keeps read access to
	// what happened, but nothing new can be submitted for them (#2487).
	if err := child.RequireCareRunning(); err != nil {
		return false, "", err
	}
	today, err := s.checkPickupChangeAllowed(ctx, child.TenantID, date)
	if err != nil {
		return false, "", err
	}
	if s.CareRequests == nil {
		return false, "", errors.New("parent: pickup change request service not configured")
	}
	return reasonRequired, today, nil
}

// checkPickupChangeAllowed verifies the school allows the one-day pickup
// change and the date lies in the bookable window. It returns today's date.
func (s *Service) checkPickupChangeAllowed(ctx context.Context, tenantID int64, date calendar.Date) (calendar.Date, error) {
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, KeyParentPickupChangeEnabled)
	if err != nil {
		return "", fmt.Errorf("parent: resolve pickup-change setting: %w", err)
	}
	if !enabled {
		return "", ErrPickupChangeDisabled
	}
	today := s.todayDate()
	if date.Before(today) {
		return "", ErrPastCareDate
	}
	if date.After(calendar.NewDate(today.Year(), today.Month()+2, today.Day())) {
		return "", ErrCareDateTooFar
	}
	return today, nil
}

// checkDayDepartureModeAllowed gates the Abholart leg of a one-day request
// (#3945). It needs the one-day change (checked by the caller) and the
// school's permission to ask for a different Abholart at all, so a school
// that keeps the Abholart with the OGS does not get it through the day path.
func (s *Service) checkDayDepartureModeAllowed(ctx context.Context, tenantID int64, date calendar.Date, departureMode string) error {
	if departureMode == "" {
		return nil
	}
	careDay, err := calendar.IsCareWeekday(ctx, calendar.Date(date))
	if err != nil {
		return fmt.Errorf("parent: resolve weekend plan for departure mode: %w", err)
	}
	if !careDay {
		return ErrCareExceptionNotCareDay
	}
	if s.Settings == nil {
		return errors.New("parent: settings service not configured")
	}
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, KeyParentCareModeRequestEnabled)
	if err != nil {
		return fmt.Errorf("parent: resolve departure-mode request setting: %w", err)
	}
	if !enabled {
		return ErrCareRequestFieldDisabled
	}
	return nil
}

// pickupChangeLegs names what a one-day request changes.
type pickupChangeLegs struct {
	pickupTime    bool
	departureMode bool
}

// lockPickupChangeDay locks the student and the exception day, then refuses a
// day the child's care has ended on, a leg the OGS already set for that day,
// or a day the child has already left.
func (s *Service) lockPickupChangeDay(txCtx context.Context, studentID int64, date, today calendar.Date, legs pickupChangeLegs) error {
	student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
	if err != nil {
		return err
	}
	if student.CareEndedOn(s.todayDate()) {
		return ErrChildCareEnded
	}
	if err := s.CareExceptions.LockStudentAndExceptionDay(txCtx, studentID, date.String()); err != nil {
		return err
	}
	if legs.pickupTime {
		staffOwned, checkErr := s.pickupHasStaffException(txCtx, studentID, date)
		if checkErr != nil {
			return checkErr
		}
		if staffOwned {
			return ErrCareExceptionConflict
		}
	}
	if legs.departureMode {
		set, checkErr := s.departureExceptionForDate(txCtx, studentID, calendar.Date(date))
		if checkErr != nil {
			return checkErr
		}
		if set != nil {
			return ErrCareExceptionConflict
		}
	}
	alreadyLeft, checkErr := s.childAlreadyLeftToday(txCtx, studentID, date, today)
	if checkErr != nil {
		return checkErr
	}
	if alreadyLeft {
		return ErrCareExceptionAlreadyLeft
	}
	return nil
}
