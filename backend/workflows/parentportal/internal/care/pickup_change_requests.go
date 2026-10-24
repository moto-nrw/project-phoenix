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
	return s.EditPickupChange(ctx, accountID, studentID, requestID, PickupChangeEdit{
		Date: date, PickupTime: pickupTime, DepartureMode: departureMode, Reason: reason, ExpectedVersion: expectedVersion,
	})
}

// PickupChangeEdit is the new content of a pending one-day request. A
// collector or walking home alone needs a fresh signature (#4780).
type PickupChangeEdit struct {
	Date            calendar.Date
	PickupTime      time.Time
	DepartureMode   string
	Reason          string
	ExpectedVersion string
	PickupPersonID  int64
	Signature       *PickupSignatureInput
}

// EditPickupChange rewrites the caller's own pending one-day request.
func (s *Service) EditPickupChange(ctx context.Context, accountID, studentID, requestID int64, edit PickupChangeEdit) (*carerequests.Request, error) {
	if err := requireDaySignature(edit.DepartureMode, edit.PickupPersonID, edit.Signature); err != nil {
		return nil, err
	}
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, GuardianPermissionPickupManage)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if err := s.checkDayDepartureModeAllowed(ctx, child.TenantID, edit.Date, edit.DepartureMode); err != nil {
		return nil, err
	}
	if s.CareRequests == nil {
		return nil, errors.New("parent: pickup change request service not configured")
	}
	// The reason is mandatory only while the school asks the family for one
	// (#2267, story 28).
	reasonRequired := s.guardianReasonRequired(ctx, child.TenantID)
	var out *carerequests.Request
	txErr := s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		req, editErr := s.editPickupChangeInTx(txCtx, child, accountID, studentID, requestID, edit, reasonRequired)
		out = req
		return editErr
	})
	if txErr != nil {
		return nil, MapCareRequestError(txErr, "edit pickup change request")
	}
	return out, nil
}

// editPickupChangeInTx locks the child, signs a collector or the way home
// alone and rewrites the pending request inside the tenant transaction.
func (s *Service) editPickupChangeInTx(txCtx context.Context, child *Child, accountID, studentID, requestID int64, edit PickupChangeEdit, reasonRequired bool) (*carerequests.Request, error) {
	student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
	if err != nil {
		return nil, err
	}
	if student.CareEndedOn(s.todayDate()) {
		return nil, ErrChildCareEnded
	}
	cutoff, err := s.pickupChangeCutoffInTx(txCtx, child.TenantID)
	if err != nil {
		return nil, err
	}
	person, signed, err := s.signDayPickup(txCtx, child, accountID, studentID, edit.Date, edit.DepartureMode, edit.PickupPersonID, edit.Signature)
	if err != nil {
		return nil, err
	}
	return s.CareRequests.EditRequest(txCtx, carerequests.EditInput{
		PickupPerson:      person,
		Signature:         signed,
		RequestID:         requestID,
		StudentID:         studentID,
		GuardianAccountID: accountID,
		ExpectedVersion:   edit.ExpectedVersion,
		Date:              edit.Date,
		PickupTime:        edit.PickupTime,
		DepartureMode:     edit.DepartureMode,
		Reason:            edit.Reason,
		ReasonRequired:    reasonRequired,
		Cutoff:            cutoff,
	})
}

// SubmitPickupChangeRequest asks the OGS for a different Gehzeit, a different
// Abholart (#3945), or both, on one day. A zero pickupTime or an empty
// departureMode leaves that leg of the day unchanged.
func (s *Service) SubmitPickupChangeRequest(ctx context.Context, accountID, studentID int64, date calendar.Date, pickupTime time.Time, departureMode, reason string, recipientGuardianProfileIDs []int64) (*carerequests.Request, error) {
	return s.SubmitPickupChange(ctx, accountID, studentID, PickupChangeSubmission{
		Date: date, PickupTime: pickupTime, DepartureMode: departureMode, Reason: reason,
		RecipientGuardianProfileIDs: recipientGuardianProfileIDs,
	})
}

// PickupChangeSubmission is one one-day request: a Gehzeit, an Abholart, a
// contact person who collects the child (#4780), or several of them. A
// collector or walking home alone needs a signature.
type PickupChangeSubmission struct {
	Date                        calendar.Date
	PickupTime                  time.Time
	DepartureMode               string
	Reason                      string
	RecipientGuardianProfileIDs []int64
	PickupPersonID              int64
	Signature                   *PickupSignatureInput
}

// SubmitPickupChange asks the OGS for the changes of one day.
func (s *Service) SubmitPickupChange(ctx context.Context, accountID, studentID int64, submission PickupChangeSubmission) (*carerequests.Request, error) {
	submission.Reason = strings.TrimSpace(submission.Reason)
	if err := validatePickupChangeShape(submission.PickupTime, submission.DepartureMode, submission.PickupPersonID, submission.Reason); err != nil {
		return nil, err
	}
	if err := requireDaySignature(submission.DepartureMode, submission.PickupPersonID, submission.Signature); err != nil {
		return nil, err
	}
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, GuardianPermissionPickupManage)
	if err != nil {
		return nil, err
	}
	reasonRequired, today, err := s.checkPickupChangeSubmittable(ctx, child, submission.Reason, submission.Date)
	if err != nil {
		return nil, err
	}
	if err := s.checkDayDepartureModeAllowed(ctx, child.TenantID, submission.Date, submission.DepartureMode); err != nil {
		return nil, err
	}
	var result *carerequests.Request
	err = s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		created, createErr := s.submitPickupChangeInTx(txCtx, child, accountID, studentID, submission, reasonRequired, today)
		result = created
		return createErr
	})
	if err != nil {
		return nil, MapCareRequestError(err, "submit pickup change request")
	}
	return result, nil
}

// submitPickupChangeInTx locks the day, signs a collector or the way home
// alone, stores the request and its share in one tenant transaction.
func (s *Service) submitPickupChangeInTx(txCtx context.Context, child *Child, accountID, studentID int64, submission PickupChangeSubmission, reasonRequired bool, today calendar.Date) (*carerequests.Request, error) {
	legs := pickupChangeLegs{pickupTime: !submission.PickupTime.IsZero(), departureMode: submission.DepartureMode != ""}
	if err := s.lockPickupChangeDay(txCtx, studentID, submission.Date, today, legs); err != nil {
		return nil, err
	}
	policy, err := s.pickupChangePolicyInTx(txCtx, child.TenantID)
	if err != nil {
		return nil, err
	}
	if !policy.enabled {
		return nil, ErrPickupChangeDisabled
	}
	person, signed, err := s.signDayPickup(txCtx, child, accountID, studentID, submission.Date, submission.DepartureMode, submission.PickupPersonID, submission.Signature)
	if err != nil {
		return nil, err
	}
	created, err := s.CareRequests.CreatePickupChange(txCtx, carerequests.PickupChangeCreateInput{
		PickupPerson:      person,
		Signature:         signed,
		StudentID:         studentID,
		GuardianAccountID: accountID,
		Date:              submission.Date,
		PickupTime:        submission.PickupTime,
		DepartureMode:     submission.DepartureMode,
		Reason:            submission.Reason,
		// The reason is mandatory only while the school asks the family
		// for one (#2267, story 28).
		ReasonRequired: reasonRequired,
		Cutoff:         policy.cutoff,
	})
	if err != nil {
		return nil, err
	}
	// Same transaction as the request row, so a refused share never leaves
	// a request the family cannot see (#2267).
	if err := s.RequestSharing.ShareRequestInTx(
		txCtx, accountID, studentID, RequestSharePickupChange, created.ID, submission.RecipientGuardianProfileIDs,
	); err != nil {
		return nil, err
	}
	return created, nil
}

// validatePickupChangeShape runs the cheap input checks that need no tenant.
func validatePickupChangeShape(pickupTime time.Time, departureMode string, pickupPersonID int64, reason string) error {
	if pickupTime.IsZero() && departureMode == "" && pickupPersonID == 0 {
		return ErrNoCareException
	}
	if pickupPersonID < 0 || (pickupPersonID > 0 && !carerequests.PickupPersonAllowed(departureMode)) {
		return ErrInvalidCareRequestPayload
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

// requireDaySignature refuses an unsigned one-day request that names a
// collector or lets the child walk home alone (#4780).
func requireDaySignature(departureMode string, pickupPersonID int64, signature *PickupSignatureInput) error {
	if (pickupPersonID > 0 || departureMode == "alone") && signature == nil {
		return ErrPickupSignatureRequired
	}
	return nil
}

// signDayPickup resolves the collector among the child's contacts and stores
// the signature the day needs, inside the caller's tenant transaction. A day
// without collector and without walking home alone returns no signature; one
// sent along is ignored.
func (s *Service) signDayPickup(txCtx context.Context, child *Child, accountID, studentID int64, date calendar.Date, departureMode string, pickupPersonID int64, signature *PickupSignatureInput) (*carerequests.PickupPerson, *carerequests.SignedStatement, error) {
	var person *carerequests.PickupPerson
	if pickupPersonID > 0 {
		resolved, err := s.childContactPerson(txCtx, studentID, pickupPersonID)
		if err != nil {
			return nil, nil, err
		}
		person = resolved
	}
	personName := ""
	if person != nil {
		personName = person.Name
	}
	statement := DayPickupStatement(child.StudentName, date, departureMode, personName)
	if statement == "" {
		return person, nil, nil
	}
	signed, err := s.recordPickupSignature(txCtx, studentID, accountID, statement, signature)
	if err != nil {
		return nil, nil, err
	}
	return person, signed, nil
}

// childContactPerson finds a contact person linked to the child.
func (s *Service) childContactPerson(ctx context.Context, studentID, guardianProfileID int64) (*carerequests.PickupPerson, error) {
	links, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		if link == nil || link.GuardianProfileID != guardianProfileID {
			continue
		}
		profile, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID)
		if errors.Is(err, ErrGuardianProfileNotFound) {
			return nil, ErrPickupPersonNotLinked
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(profile.FirstName + " " + profile.LastName)
		if name == "" {
			return nil, ErrPickupPersonNotLinked
		}
		return &carerequests.PickupPerson{GuardianProfileID: guardianProfileID, Name: name}, nil
	}
	return nil, ErrPickupPersonNotLinked
}
