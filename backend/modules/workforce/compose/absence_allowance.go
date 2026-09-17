package compose

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// allowanceUses spreads an absence over the weekdays it covers. Half-day
// flags apply only at the original boundaries; weekends never consume
// allowance. The booking's entry day decides whether an expired rest may
// still be drawn on.
func allowanceUses(absence domain.StaffAbsence) ([]domain.AllowanceUse, error) {
	start, err := timezone.ParseDate(absence.DateStart)
	if err != nil {
		return nil, err
	}
	end, err := timezone.ParseDate(absence.DateEnd)
	if err != nil {
		return nil, err
	}
	startHalf, endHalf := absence.StartHalfDay, absence.EndHalfDay
	if absence.HalfDay && !startHalf && !endHalf {
		startHalf, endHalf = true, true
	}
	pending := absence.Status == domain.AbsenceStatusRequested || absence.Status == domain.AbsenceStatusQuestion
	enteredOn := timezone.DateFromTime(absence.CreatedAt).String()
	uses := make([]domain.AllowanceUse, 0, max(0, start.DaysUntil(end)+1))
	for day := start; !day.After(end); day = day.AddDays(1) {
		if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
			continue
		}
		days := 1.0
		if day == start && startHalf || day == end && endHalf {
			days = 0.5
		}
		uses = append(uses, domain.AllowanceUse{
			AbsenceID: absence.ID, Day: day.String(), Days: days, Pending: pending, EnteredOn: enteredOn,
		})
	}
	return uses, nil
}

func allowanceSummaryToPublic(value domain.AbsenceTypeAllowanceSummary) workforce.AbsenceTypeAllowanceSummary {
	result := workforce.AbsenceTypeAllowanceSummary{
		StaffID: value.StaffID, AbsenceTypeID: value.AbsenceTypeID, Year: value.Year,
		EntitledDays: value.EntitledDays, TakenDays: value.TakenDays, ReservedDays: value.ReservedDays, RemainingDays: value.RemainingDays,
		ExpiresOn: value.ExpiresOn, ExpiredDays: value.ExpiredDays, BookingDays: value.BookingDays,
	}
	if carried := value.CarriedIn; carried != nil {
		result.CarriedIn = &workforce.AbsenceTypeAllowanceCarry{
			Year: carried.Year, RemainingDays: carried.RemainingDays, ExpiredDays: carried.ExpiredDays, ExpiresOn: carried.ExpiresOn,
		}
	}
	return result
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
		result = append(result, allowanceSummaryToPublic(value))
	}
	return result, mapError(err)
}
