package timetracking

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (s *staffAbsenceService) GetVacationQuotaSummary(ctx context.Context, staffID int64, year int) (*VacationQuotaSummary, error) {
	in, err := s.loadVacationQuotaInputs(ctx, staffID, year)
	if err != nil {
		return nil, err
	}
	return computeVacationQuotaSummary(staffID, year, in.entitled, in.carryover, in.absences, in.opening), nil
}

type vacationQuotaInputs struct {
	entitled, carryover float64
	absences            []*StaffAbsence
	opening             *StaffVacationOpening
}

func (s *staffAbsenceService) loadVacationQuotaInputs(ctx context.Context, staffID int64, year int) (vacationQuotaInputs, error) {
	in := vacationQuotaInputs{entitled: defaultEntitledDays}
	// Bare-constructed services (unit tests) have no quota repository and
	// fall back to the default entitlement.
	if s.quotaRepo != nil {
		quota, err := s.quotaRepo.GetByStaffAndYear(ctx, staffID, year)
		if err != nil {
			return vacationQuotaInputs{}, fmt.Errorf("failed to fetch quota: %w", err)
		}
		if quota != nil {
			in.entitled = quota.EntitledDays
			in.carryover = quota.CarryoverDays
		}
	}

	yearStart := calendar.NewDate(year, time.January, 1)
	yearEnd := calendar.NewDate(year, time.December, 31)
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, yearStart, yearEnd)
	if err != nil {
		return vacationQuotaInputs{}, fmt.Errorf("failed to fetch year absences: %w", err)
	}
	in.absences = absences
	if s.openingRepo != nil {
		in.opening, err = s.openingRepo.GetByStaffAndYear(ctx, staffID, year)
		if err != nil {
			return vacationQuotaInputs{}, fmt.Errorf("failed to fetch vacation opening: %w", err)
		}
	}
	return in, nil
}

// ensureVacationFits rejects candidate when it would push the Resturlaub of
// any calendar year it touches below zero. excludeID leaves the candidate's
// own stored row out of the account; withReserved also counts other open
// requests, which a new booking must leave room for.
func (s *staffAbsenceService) ensureVacationFits(ctx context.Context, candidate *StaffAbsence, excludeID int64, withReserved bool) error {
	for year := candidate.DateStart.Year(); year <= candidate.DateEnd.Year(); year++ {
		in, err := s.loadVacationQuotaInputs(ctx, candidate.StaffID, year)
		if err != nil {
			return err
		}
		counted := countedVacationAbsences(in.absences, excludeID, withReserved)
		summary := computeVacationQuotaSummary(candidate.StaffID, year, in.entitled, in.carryover, counted, in.opening)
		needed := vacationDaysForYear(candidate, year)
		if needed > summary.RemainingDays+vacationDayEpsilon {
			return fmt.Errorf("%w: %s", ErrVacationQuotaExceeded, formatVacationShortfall(year, summary.RemainingDays, needed))
		}
	}
	return nil
}

// countedVacationAbsences is the part of a year's absences the account of a
// candidate booking counts: without the candidate's own stored row
// (excludeID) and, unless withReserved, without the open requests.
func countedVacationAbsences(absences []*StaffAbsence, excludeID int64, withReserved bool) []*StaffAbsence {
	counted := make([]*StaffAbsence, 0, len(absences))
	for _, a := range absences {
		if a.ID == excludeID && excludeID != 0 {
			continue
		}
		if !withReserved && (a.Status == AbsenceStatusRequested || a.Status == AbsenceStatusQuestion) {
			continue
		}
		counted = append(counted, a)
	}
	return counted
}

// vacationDayEpsilon absorbs float noise from half-day sums.
const vacationDayEpsilon = 0.001

func formatVacationShortfall(year int, remaining, needed float64) string {
	return fmt.Sprintf("%d: remaining %s, needed %s",
		year, strconv.FormatFloat(remaining, 'f', -1, 64), strconv.FormatFloat(needed, 'f', -1, 64))
}

// createDirectVacation books vacation the Leitung agreed with the staff
// member (#3256): no request, status reported, counted as taken right away.
// It never merges into neighbouring entries; any overlap is rejected like a
// vacation request.
func (s *staffAbsenceService) createDirectVacation(ctx context.Context, staffID, createdBy int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error) {
	dateStart, dateEnd, err := parseDateRange(req.DateStart, req.DateEnd)
	if err != nil {
		return nil, err
	}
	if req.HalfDay && dateStart != dateEnd {
		return nil, &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceInvalid, Cause: fmt.Errorf("invalid vacation absence: half-day entries must cover exactly one date")}
	}
	if err := s.lockStaffAbsenceWrites(ctx, staffID); err != nil {
		return nil, err
	}
	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, dateStart, dateEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing absences: %w", err)
	}
	if len(filterBlockingAbsences(existing)) > 0 {
		return nil, &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceDatesOverlap, Cause: fmt.Errorf("dates overlap with an existing absence")}
	}
	workingDays := countWorkingDays(dateStart, dateEnd, req.HalfDay, req.HalfDay)
	if workingDays <= 0 {
		return nil, workforce.ErrAbsenceNoWorkingDays
	}

	now := time.Now()
	absence := &StaffAbsence{
		StaffID:      staffID,
		AbsenceType:  AbsenceTypeVacation,
		DateStart:    dateStart,
		DateEnd:      dateEnd,
		HalfDay:      req.HalfDay,
		StartHalfDay: req.HalfDay,
		EndHalfDay:   req.HalfDay,
		Note:         req.Note,
		Status:       AbsenceStatusReported,
		CreatedBy:    createdBy,
		WorkingDays:  &workingDays,
		RequestedAt:  now,
	}
	absence.CreatedAt = now
	absence.UpdatedAt = now
	absence.SetTenantID(tenant.FromContext(ctx))

	if err := s.rejectVacationBeforeOpening(ctx, absence); err != nil {
		return nil, err
	}
	if err := s.ensureVacationFits(ctx, absence, 0, true); err != nil {
		return nil, err
	}
	s.warnIfWorkSessionsExist(ctx, staffID, dateStart, dateEnd)
	if err := s.absenceRepo.Create(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to create vacation: %w", err)
	}
	s.broadcastTimeTrackingChanged(ctx)
	return s.withLabel(ctx, toAbsenceResponse(absence)), nil
}

// computeVacationQuotaSummary is the pure tail of GetVacationQuotaSummary: the
// two reads happen at the call site, the arithmetic lives here. Extracted so
// the cross-staff overview (#1417) can compute Resturlaub from prefetched rows
// through the SAME code, instead of a second implementation that would drift.
func computeVacationQuotaSummary(
	staffID int64,
	year int,
	entitled, carryover float64,
	absences []*StaffAbsence,
	opening *StaffVacationOpening,
) *VacationQuotaSummary {
	return computeVacationQuotaSummaryThrough(
		staffID,
		year,
		entitled,
		carryover,
		calendar.NewDate(year, time.December, 31),
		absences,
		opening,
	)
}

// computeVacationQuotaSummaryThrough is the historical counterpart of the
// full-year quota summary. Absence ranges are clipped at through so a past
// month does not spend vacation that lies later in the selected year.
func computeVacationQuotaSummaryThrough(
	staffID int64,
	year int,
	entitled, carryover float64,
	through calendar.Date,
	absences []*StaffAbsence,
	opening *StaffVacationOpening,
) *VacationQuotaSummary {
	yearStart := calendar.NewDate(year, time.January, 1)
	yearEnd := calendar.NewDate(year, time.December, 31)
	if through.Before(yearEnd) {
		yearEnd = through
	}

	// The takeover (#2132) counts only in its own year and only from its
	// Stichtag on — a historical view that ends before the introduction
	// keeps the pre-moto arithmetic.
	takenBefore := 0.0
	if opening != nil && opening.Year == year && !through.Before(opening.EffectiveDate) {
		takenBefore = opening.TakenBeforeDays
	} else {
		opening = nil
	}

	taken, reserved := 0.0, 0.0
	for _, a := range absences {
		if a.AbsenceType != AbsenceTypeVacation {
			continue
		}
		days := vacationDaysWithin(a, yearStart, yearEnd)
		switch a.Status {
		case AbsenceStatusApproved, AbsenceStatusReported:
			taken += days
		case AbsenceStatusRequested, AbsenceStatusQuestion:
			reserved += days
		}
	}

	return &VacationQuotaSummary{
		StaffID:         staffID,
		Year:            year,
		EntitledDays:    entitled,
		CarryoverDays:   carryover,
		TakenBeforeDays: takenBefore,
		TakenDays:       taken,
		ReservedDays:    reserved,
		RemainingDays:   entitled + carryover - takenBefore - taken - reserved,
		Opening:         opening,
	}
}

// vacationDaysWithin is what one vacation row spends between yearStart and
// yearEnd: its stored working days when it lies inside, else the clipped count.
func vacationDaysWithin(a *StaffAbsence, yearStart, yearEnd calendar.Date) float64 {
	if a.WorkingDays != nil && dateWithinRange(a.DateStart, yearStart, yearEnd) && dateWithinRange(a.DateEnd, yearStart, yearEnd) {
		return *a.WorkingDays
	}
	return vacationDaysInYear(a, yearStart, yearEnd)
}

func vacationDaysForYear(a *StaffAbsence, year int) float64 {
	return vacationDaysWithin(a, calendar.NewDate(year, time.January, 1), calendar.NewDate(year, time.December, 31))
}

func dateWithinRange(d, from, to calendar.Date) bool {
	return !d.Before(from) && !d.After(to)
}

func vacationDaysInYear(a *StaffAbsence, yearStart, yearEnd calendar.Date) float64 {
	from := a.DateStart
	if from.Before(yearStart) {
		from = yearStart
	}
	to := a.DateEnd
	if to.After(yearEnd) {
		to = yearEnd
	}
	if to.Before(from) {
		return 0
	}

	startHalf, endHalf := effectiveBoundaryHalfDays(a)
	if from != a.DateStart {
		startHalf = false
	}
	if to != a.DateEnd {
		endHalf = false
	}
	return countWorkingDays(from, to, startHalf, endHalf)
}

// VacationQuotaChange is a reasoned change of one staff member's yearly
// vacation entitlement.
type VacationQuotaChange struct {
	StaffID       int64
	Year          int
	EntitledDays  float64
	CarryoverDays float64
	Reason        string
	ChangedBy     int64
}

func (s *staffAbsenceService) UpsertVacationQuota(ctx context.Context, staffID int64, year int, entitled, carryover float64) error {
	return s.upsertVacationQuota(ctx, VacationQuotaChange{StaffID: staffID, Year: year, EntitledDays: entitled, CarryoverDays: carryover})
}

func (s *staffAbsenceService) SetVacationQuota(ctx context.Context, change VacationQuotaChange) error {
	if strings.TrimSpace(change.Reason) == "" || change.ChangedBy <= 0 {
		return fmt.Errorf("%w: reason and actor are required", ErrVacationQuotaInvalid)
	}
	return s.upsertVacationQuota(ctx, change)
}

func (s *staffAbsenceService) upsertVacationQuota(ctx context.Context, change VacationQuotaChange) error {
	staffID, year, entitled, carryover := change.StaffID, change.Year, change.EntitledDays, change.CarryoverDays
	now := time.Now()
	quota := &StaffVacationQuota{
		StaffID:       staffID,
		Year:          year,
		EntitledDays:  entitled,
		CarryoverDays: carryover,
		ChangeReason:  strings.TrimSpace(change.Reason),
		ChangedBy:     change.ChangedBy,
	}
	quota.CreatedAt = now
	quota.UpdatedAt = now
	quota.SetTenantID(tenant.FromContext(ctx))
	if err := quota.Validate(); err != nil {
		return fmt.Errorf("%w: %s", ErrVacationQuotaInvalid, err.Error())
	}
	if err := s.lockStaffAbsenceWrites(ctx, staffID); err != nil {
		return err
	}
	// The claim may not drop below what is already taken or requested
	// (#3256): the Resturlaub would turn negative.
	in, err := s.loadVacationQuotaInputs(ctx, staffID, year)
	if err != nil {
		return err
	}
	if summary := computeVacationQuotaSummary(staffID, year, entitled, carryover, in.absences, in.opening); summary.RemainingDays < -vacationDayEpsilon {
		return fmt.Errorf("%w: %d: remaining %s", ErrVacationQuotaExceeded, year,
			strconv.FormatFloat(summary.RemainingDays, 'f', -1, 64))
	}
	if err := s.quotaRepo.Upsert(ctx, quota); err != nil {
		return err
	}
	s.broadcastTimeTrackingChanged(ctx)
	return nil
}
