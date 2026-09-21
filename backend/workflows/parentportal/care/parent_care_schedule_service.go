// Parent-portal read view + request lifecycle for the child's permanent
// weekly care plan (#1803). The read view lives on the Stammdaten page; a
// change request is created and withdrawn THERE (not in the chat) and decided
// by staff on the central Änderungsanfragen page. The chat only receives
// notification pills, emitted by the schedule-domain request service.
package care

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
)

var (
	// ErrCareRequestNotFound means no pending request matched under this child.
	ErrCareRequestNotFound = errors.New("parent: care schedule request not found")
	// ErrCareRequestNotPending means the request was already decided/withdrawn.
	ErrCareRequestNotPending = errors.New("parent: care schedule request is not pending")
	// ErrCareRequestAlreadyPending means the child already has an open request.
	ErrCareRequestAlreadyPending = errors.New("parent: care schedule request already pending")
	// ErrInvalidCareRequestPayload means the payload failed validation.
	ErrInvalidCareRequestPayload = errors.New("parent: invalid care schedule request payload")
	// ErrCareRequestFieldDisabled means the payload contains at least one field
	// group the child's school has disabled for permanent parent requests.
	ErrCareRequestFieldDisabled = errors.New("parent: care schedule request field disabled")
	// ErrCareRequestBookingsAuthoritative means the school runs booking-led care
	// (enrollment.bookings_authoritative), so permanent weekly-plan requests are
	// not a parent path there at all (#2793).
	ErrCareRequestBookingsAuthoritative = errors.New("parent: care schedule requests unavailable under booking-led care")
)

// CareScheduleRequestCapabilities are the resolved field-level permissions
// returned to the parent UI and enforced again on submission.
type CareScheduleRequestCapabilities struct {
	Arrival       bool
	Pickup        bool
	DepartureMode bool
}

func (c CareScheduleRequestCapabilities) Any() bool {
	return c.Pickup || c.DepartureMode
}

// CareScheduleWeekday is one weekday (1=Mon..5=Fri) of the child's current
// permanent weekly plan: arrival/pickup wall-clock times (empty = not set) and
// the allowed departure-mode keys (an "alone" entry stands in for an empty
// configured set, matching the diff convention).
type CareScheduleWeekday struct {
	Weekday int
	Status  careplan.CareDayStatus
	Arrival string
	Pickup  string
	Modes   []string
}

// PendingCareRequest is the child's open change request as shown on the
// Stammdaten read view: the live "current → requested" diff plus whether the
// CALLING guardian submitted it (only the submitter may withdraw).
type PendingCareRequest struct {
	ID              int64
	CreatedAt       time.Time
	Diff            []carerequests.DiffEntry
	SubmittedBySelf bool
}

// ChildCareSchedule is the parent-facing weekly care plan view.
type ChildCareSchedule struct {
	Weekdays       []CareScheduleWeekday
	PendingRequest *PendingCareRequest
	// CanRequest gates the "Änderung anfragen" button: at least one field is
	// enabled and the calling guardian holds parent_portal.request.submit.
	CanRequest          bool
	RequestCapabilities CareScheduleRequestCapabilities
	// TodayAbsent is true when the child has any active scheduled absence today
	// (sick, excused, or class trip — any source). It is the parent-safe absence
	// signal the "Heute → Abholung" tile needs: ListSickDays deliberately hides
	// staff-created excused and class-trip days, so the tile cannot infer "the
	// child is off today" from that list alone. Only a boolean is exposed — no
	// note, source, or status leaks to the guardian.
	TodayAbsent bool
	// TodayDate is the Berlin calendar day TodayAbsent was resolved against —
	// the server's "today" at request-handling time. The client stamps its
	// cached absence signal with THIS date (not the browser's request-start
	// day) so a request that crosses Berlin midnight cannot bind the new day's
	// today_absent to yesterday's pickup tile (#1725 review).
	TodayDate timezone.Date
}

// GetChildCareSchedule returns the child's current weekly care plan with any
// pending change request. Authorization only (parent_portal.access) — the
// read view stays available regardless of the request feature gates.
func (s *Service) GetChildCareSchedule(ctx context.Context, accountID, studentID int64) (*ChildCareSchedule, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	view := &ChildCareSchedule{}
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		if err := s.buildCareScheduleView(txCtx, view, accountID, studentID); err != nil {
			return err
		}
		// Resolve "today" once and carry it on the view so the wire response can
		// report the exact day today_absent was computed against (#1725 review).
		today := s.todayDate()
		absent, err := s.hasActiveAbsenceToday(txCtx, studentID, today)
		if err != nil {
			return err
		}
		view.TodayAbsent = absent
		view.TodayDate = today
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: get child care schedule: %w", txErr)
	}
	capabilities, _, err := s.resolveCareScheduleRequestCapabilities(ctx, child.TenantID)
	if err != nil {
		return nil, err
	}
	view.RequestCapabilities = capabilities
	view.CanRequest = child.HasPermission(authorize.GuardianPermissionRequestSubmit) && capabilities.Any()
	return view, nil
}

// CreateCareScheduleRequest stores a pending change request for the child's
// weekly plan and returns the refreshed view. Requires
// parent_portal.request.submit and the field-level school settings. Messaging
// is independent: notification pills are best-effort when messages are on.
func (s *Service) CreateCareScheduleRequest(ctx context.Context, accountID, studentID int64, payload map[string]any) (*ChildCareSchedule, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionRequestSubmit)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to what
	// happened, but nothing new can be submitted for them (#2487).
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	capabilities, bookingsAuthoritative, err := s.resolveCareScheduleRequestCapabilities(ctx, child.TenantID)
	if err != nil {
		return nil, err
	}
	if bookingsAuthoritative {
		return nil, ErrCareRequestBookingsAuthoritative
	}
	raw, requested, err := encodeRequestedCareFields(payload)
	if err != nil {
		return nil, MapCareRequestError(err, "validate care schedule request")
	}
	if (requested.Arrival && !capabilities.Arrival) ||
		(requested.Pickup && !capabilities.Pickup) ||
		(requested.DepartureMode && !capabilities.DepartureMode) ||
		(requested.Scheduled && (!capabilities.Pickup || !capabilities.DepartureMode)) {
		return nil, ErrCareRequestFieldDisabled
	}
	view := &ChildCareSchedule{CanRequest: capabilities.Any(), RequestCapabilities: capabilities}
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if student.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		if _, err := s.CareRequests.CreateRequest(txCtx, studentID, accountID, raw); err != nil {
			return err
		}
		return s.buildCareScheduleView(txCtx, view, accountID, studentID)
	})
	if txErr != nil {
		return nil, MapCareRequestError(txErr, "create care schedule request")
	}
	s.Logger.Info("parent created care schedule request",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.TenantID),
	)
	return view, nil
}

func encodeRequestedCareFields(payload map[string]any) (json.RawMessage, carerequests.RequestedFieldGroups, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, carerequests.RequestedFieldGroups{}, carerequests.ErrInvalidPayload
	}
	fields, err := carerequests.RequestedFields(raw)
	return raw, fields, err
}

// resolveCareScheduleRequestCapabilities returns the field-level capabilities
// and whether the school runs booking-led care. Under booking-led care the
// capabilities are always empty; the flag lets the create path name that as
// the reason instead of a disabled field.
func (s *Service) resolveCareScheduleRequestCapabilities(ctx context.Context, tenantID int64) (CareScheduleRequestCapabilities, bool, error) {
	resolve := func(key string) (bool, error) {
		if s.Settings == nil {
			return false, errors.New("parent: settings service not configured")
		}
		return s.Settings.ResolveBoolForTenant(ctx, tenantID, key)
	}
	bookingsAuthoritative, err := resolve(configModels.KeyEnrollmentBookingsAuthoritative)
	if err != nil {
		return CareScheduleRequestCapabilities{}, false, fmt.Errorf("parent: resolve bookings-authoritative setting: %w", err)
	}
	if bookingsAuthoritative {
		return CareScheduleRequestCapabilities{}, true, nil
	}
	pickup, err := resolve(configModels.KeyParentCarePickupRequestEnabled)
	if err != nil {
		return CareScheduleRequestCapabilities{}, false, fmt.Errorf("parent: resolve pickup request setting: %w", err)
	}
	mode, err := resolve(configModels.KeyParentCareModeRequestEnabled)
	if err != nil {
		return CareScheduleRequestCapabilities{}, false, fmt.Errorf("parent: resolve departure-mode request setting: %w", err)
	}
	return CareScheduleRequestCapabilities{Pickup: pickup, DepartureMode: mode}, false, nil
}

// buildCareScheduleView loads the weekly plan + pending request inside the
// caller's tenant transaction.
func (s *Service) buildCareScheduleView(ctx context.Context, view *ChildCareSchedule, accountID, studentID int64) error {
	if s.StudentRepo == nil || s.ArrivalSchedules == nil || s.PickupSchedules == nil || s.CareRequests == nil {
		return errors.New("parent: care schedule dependencies not wired")
	}
	student, err := s.StudentRepo.FindByID(ctx, studentID)
	if err != nil {
		return err
	}
	arrivals, err := s.ArrivalSchedules.GetStudentArrivalSchedules(ctx, studentID)
	if err != nil {
		return err
	}
	pickups, err := s.PickupSchedules.GetStudentPickupSchedules(ctx, studentID)
	if err != nil {
		return err
	}

	arrivalByDay := map[int]string{}
	arrivalCareDays := map[int]bool{}
	for _, a := range arrivals {
		arrivalCareDays[a.Weekday] = true
		if a.ExpectedArrival.IsZero() {
			// Care day without a time: the class carries none yet (#2414).
			continue
		}
		arrivalByDay[a.Weekday] = a.ExpectedArrival.Format("15:04")
	}
	pickupByDay := map[int]string{}
	for _, p := range pickups {
		pickupByDay[p.Weekday] = p.PickupTime.Format("15:04")
	}
	hasCarePlan := len(arrivals) > 0 || len(pickups) > 0

	weekdays := make([]CareScheduleWeekday, 0, len(usersModels.PickupDayOrder))
	for i, abbrev := range usersModels.PickupDayOrder {
		wd := i + 1
		status := careDayStatus(hasCarePlan, arrivalCareDays[wd], arrivalByDay[wd], pickupByDay[wd])
		modes := student.AllowedDepartureModes[abbrev]
		keys := make([]string, 0, len(modes))
		for _, m := range modes {
			keys = append(keys, string(m))
		}
		if len(keys) == 0 && status == careplan.CareDayScheduled {
			// An empty configured set means the child goes home alone — surface
			// that explicitly instead of an ambiguous empty list (matches the
			// request-diff convention).
			keys = []string{string(usersModels.DepartureAlone)}
		}
		weekdays = append(weekdays, CareScheduleWeekday{
			Weekday: wd,
			Status:  status,
			Arrival: arrivalByDay[wd],
			Pickup:  pickupByDay[wd],
			Modes:   keys,
		})
	}
	view.Weekdays = weekdays

	pending, diff, err := s.CareRequests.GetPendingForStudent(ctx, studentID)
	if err != nil {
		return err
	}
	if pending != nil {
		visibility, visibilityErr := s.RequestSharing.LoadRequestShareVisibility(ctx, studentID)
		if visibilityErr != nil {
			return visibilityErr
		}
		view.PendingRequest = pendingCareRequest(
			pending, diff, accountID,
			visibility.Allows(RequestShareCareSchedule, pending.ID, accountID, pending.SubmittedBy),
		)
	}
	return nil
}

func pendingCareRequest(
	pending *carerequests.Request,
	diff []carerequests.DiffEntry,
	accountID int64,
	visible bool,
) *PendingCareRequest {
	if pending == nil || !visible {
		return nil
	}
	return &PendingCareRequest{
		ID:              pending.ID,
		CreatedAt:       pending.CreatedAt,
		Diff:            diff,
		SubmittedBySelf: pending.SubmittedBy == accountID,
	}
}

func careDayStatus(hasCarePlan, hasArrivalDay bool, arrival, pickup string) careplan.CareDayStatus {
	if hasArrivalDay || arrival != "" || pickup != "" {
		return careplan.CareDayScheduled
	}
	if hasCarePlan {
		return careplan.CareDayNotScheduled
	}
	return careplan.CareDayUnknown
}

// hasActiveAbsenceToday reports whether the child has any active scheduled
// absence for today — sick, staff- or parent-excused, or a class trip — from
// active.student_status_days. It is the authoritative, parent-safe absence
// signal for the "Heute → Abholung" tile: unlike ListSickDays (which hides
// staff-created excused and class-trip rows to avoid leaking internal
// notes/source), this returns only a boolean, so no hidden status detail
// reaches the guardian. Must run inside the child's tenant transaction; a
// missing repo (unwired) yields false rather than a panic.
func (s *Service) hasActiveAbsenceToday(ctx context.Context, studentID int64, today timezone.Date) (bool, error) {
	if s.StatusDayRepo == nil {
		return false, nil
	}
	rows, err := s.StatusDayRepo.FindActiveByStudentAndDateRange(ctx, studentID, today, today)
	if err != nil {
		return false, err
	}
	for _, r := range rows {
		switch r.Status {
		case activeModels.StudentStatusDaySick,
			activeModels.StudentStatusDayExcused,
			activeModels.StudentStatusDayClassTrip:
			return true, nil
		}
	}
	return false, nil
}

// MapCareRequestError translates the schedule-domain sentinels into this
// package's parent-facing sentinels (which the handler maps to HTTP codes);
// anything else is wrapped as an internal error.
// EditCareScheduleRequest rewrites the caller's own pending weekly-plan
// request (#2267, story 37). It replaces withdrawal, so the request keeps its
// id and its share. Like the withdraw it replaces it does not re-check the
// per-field capability switches: a request already filed must stay
// correctable even after the school turns a field off — otherwise the parent
// is left with an open request they can neither fix nor retract.
func (s *Service) EditCareScheduleRequest(
	ctx context.Context, accountID, studentID, requestID int64,
	payload map[string]any, expectedVersion string,
) (*ChildCareSchedule, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if s.CareRequests == nil {
		return nil, ErrCareRequestNotFound
	}
	capabilities, _, err := s.resolveCareScheduleRequestCapabilities(ctx, child.TenantID)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, ErrInvalidCareRequestPayload
	}
	view := &ChildCareSchedule{CanRequest: capabilities.Any(), RequestCapabilities: capabilities}
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if student.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		if _, editErr := s.CareRequests.EditRequest(txCtx, carerequests.EditInput{
			RequestID:         requestID,
			StudentID:         studentID,
			GuardianAccountID: accountID,
			ExpectedVersion:   expectedVersion,
			Payload:           raw,
		}); editErr != nil {
			return editErr
		}
		return s.buildCareScheduleView(txCtx, view, accountID, studentID)
	})
	if txErr != nil {
		return nil, MapCareRequestError(txErr, "edit care schedule request")
	}
	return view, nil
}

func MapCareRequestError(err error, op string) error {
	switch {
	case errors.Is(err, carerequests.ErrNotFound):
		return ErrCareRequestNotFound
	case errors.Is(err, carerequests.ErrNotPending):
		return ErrCareRequestNotPending
	case errors.Is(err, carerequests.ErrAlreadyPending):
		return ErrCareRequestAlreadyPending
	case errors.Is(err, carerequests.ErrInvalidPayload):
		return ErrInvalidCareRequestPayload
	case errors.Is(err, careplan.ErrPickupChangeCutoffPassed):
		return ErrPickupChangeCutoffPassed
	default:
		return fmt.Errorf("parent: %s: %w", op, err)
	}
}

// CareRequestDiffEntry is one "current → requested" row of a pending
// care-schedule request, as PendingCareRequest.Diff carries it.
type CareRequestDiffEntry = carerequests.DiffEntry

// PickupChangeRequest is a guardian's one-day pickup change request.
type PickupChangeRequest = carerequests.Request
