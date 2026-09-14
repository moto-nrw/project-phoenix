package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// absenceDaysInYear clips calendar coverage to a year. Half-day flags apply
// only at the original boundaries; weekends never consume allowance.
func absenceDaysInYear(absence domain.StaffAbsence, year int) (float64, error) {
	coverage, err := absenceCoverageInYear(absence, year)
	if err != nil {
		return 0, err
	}
	days := 0.0
	for _, value := range coverage {
		days += value
	}
	return days, nil
}

func absenceCoverageInYear(absence domain.StaffAbsence, year int) (map[timezone.Date]float64, error) {
	start, err := timezone.ParseDate(absence.DateStart)
	if err != nil {
		return nil, err
	}
	end, err := timezone.ParseDate(absence.DateEnd)
	if err != nil {
		return nil, err
	}
	from, to := start, end
	yearStart := timezone.NewDate(year, time.January, 1)
	yearEnd := timezone.NewDate(year, time.December, 31)
	if from.Before(yearStart) {
		from = yearStart
	}
	if to.After(yearEnd) {
		to = yearEnd
	}
	startHalf, endHalf := absence.StartHalfDay, absence.EndHalfDay
	if absence.HalfDay && !startHalf && !endHalf {
		startHalf, endHalf = true, true
	}
	coverage := make(map[timezone.Date]float64)
	for day := from; !day.After(to); day = day.AddDays(1) {
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}
		if day == start && startHalf || day == end && endHalf {
			coverage[day] = 0.5
		} else {
			coverage[day] = 1
		}
	}
	return coverage, nil
}

// Overlapping bookings are merged by the writer. Only their maximum daily
// coverage offsets a candidate, never the sum of overlapping rows.
func additionalAbsenceDaysInYear(candidate domain.StaffAbsence, existing []domain.StaffAbsence, year int) (float64, error) {
	previous := make(map[timezone.Date]float64)
	for _, absence := range existing {
		coverage, err := absenceCoverageInYear(absence, year)
		if err != nil {
			return 0, err
		}
		for day, value := range coverage {
			previous[day] = max(previous[day], value)
		}
	}
	coverage, err := absenceCoverageInYear(candidate, year)
	if err != nil {
		return 0, err
	}
	additional := 0.0
	for day, value := range coverage {
		additional += max(0, value-previous[day])
	}
	return additional, nil
}

func (e engine) PreviewAllowanceBooking(ctx context.Context, staffID, absenceTypeID int64, start, end string, halfDay bool) ([]workforce.AbsenceTypeAllowanceSummary, error) {
	from, err := timezone.ParseDate(start)
	if err != nil || from.IsZero() {
		return nil, workforce.ErrAbsenceTypeAllowanceInvalid
	}
	to, err := timezone.ParseDate(end)
	if err != nil || to.IsZero() || to.Before(from) {
		return nil, workforce.ErrAbsenceTypeAllowanceInvalid
	}
	values, err := e.service.PreviewAllowanceBooking(ctx, staffID, absenceTypeID, domain.StaffAbsence{
		DateStart: start, DateEnd: end, HalfDay: halfDay, StartHalfDay: halfDay, EndHalfDay: halfDay,
	}, from.Year(), to.Year())
	if values == nil {
		return nil, mapError(err)
	}
	result := make([]workforce.AbsenceTypeAllowanceSummary, 0, len(values))
	for _, value := range values {
		result = append(result, workforce.AbsenceTypeAllowanceSummary{
			StaffID: value.StaffID, AbsenceTypeID: value.AbsenceTypeID, Year: value.Year,
			EntitledDays: value.EntitledDays, TakenDays: value.TakenDays, ReservedDays: value.ReservedDays, RemainingDays: value.RemainingDays,
		})
	}
	return result, mapError(err)
}
