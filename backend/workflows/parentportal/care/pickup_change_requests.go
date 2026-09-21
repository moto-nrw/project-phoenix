package care

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// EditPickupChangeRequest rewrites the caller's own pending one-day pickup
// change (#2267, story 37).
func (s *Service) EditPickupChangeRequest(
	ctx context.Context, accountID, studentID, requestID int64,
	date timezone.Date, pickupTime time.Time, reason, expectedVersion string,
) (*carerequests.Request, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPickupManage)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if s.CareRequests == nil {
		return nil, errors.New("parent: pickup change request service not configured")
	}
	var out *carerequests.Request
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
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

func (s *Service) SubmitPickupChangeRequest(ctx context.Context, accountID, studentID int64, date timezone.Date, pickupTime time.Time, reason string, recipientGuardianProfileIDs []int64) (*carerequests.Request, error) {
	reason = strings.TrimSpace(reason)
	if err := validatePickupChangeShape(pickupTime, reason); err != nil {
		return nil, err
	}
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPickupManage)
	if err != nil {
		return nil, err
	}
	reasonRequired, today, err := s.checkPickupChangeSubmittable(ctx, child, reason, date)
	if err != nil {
		return nil, err
	}
	var result *carerequests.Request
	err = InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		if err := s.lockPickupChangeDay(txCtx, studentID, date, today); err != nil {
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
func validatePickupChangeShape(pickupTime time.Time, reason string) error {
	if pickupTime.IsZero() {
		return ErrNoCareException
	}
	if utf8.RuneCountInString(reason) > 255 {
		return ErrCareExceptionReasonTooLong
	}
	return nil
}

// checkPickupChangeSubmittable runs the tenant-dependent checks that precede
// the write transaction. It returns whether the school requires a reason and
// today's date.
func (s *Service) checkPickupChangeSubmittable(ctx context.Context, child *Child, reason string, date timezone.Date) (bool, timezone.Date, error) {
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
func (s *Service) checkPickupChangeAllowed(ctx context.Context, tenantID int64, date timezone.Date) (timezone.Date, error) {
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, configModels.KeyParentPickupChangeEnabled)
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
	if date.After(timezone.NewDate(today.Year(), today.Month()+2, today.Day())) {
		return "", ErrCareDateTooFar
	}
	return today, nil
}

// lockPickupChangeDay locks the student and the exception day, then refuses a
// day the child's care has ended on, a staff-owned day, or a day the child has
// already left.
func (s *Service) lockPickupChangeDay(txCtx context.Context, studentID int64, date, today timezone.Date) error {
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
	staffOwned, checkErr := s.pickupHasStaffException(txCtx, studentID, date)
	if checkErr != nil {
		return checkErr
	}
	if staffOwned {
		return ErrCareExceptionConflict
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
