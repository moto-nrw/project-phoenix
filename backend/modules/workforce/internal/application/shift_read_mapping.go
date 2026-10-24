package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// shiftReadToPublic maps a calculation record at the public boundary.
func shiftReadToPublic(shift *workforce.ShiftRecord) workforce.StaffShift {
	if shift == nil {
		return workforce.StaffShift{}
	}
	value := workforce.StaffShift{
		ID: shift.ID, TenantID: shift.TenantID, StaffID: shift.StaffID, Date: shift.Date.String(),
		StartTime: shift.StartTime.Format(domain.ClockLayout), EndTime: shift.EndTime.Format(domain.ClockLayout), BreakMinutes: shift.BreakMinutes,
		ShiftTypeID: shift.ShiftTypeID, Notes: shift.Notes, SeriesID: shift.SeriesID, Detached: shift.Detached,
		Cancelled: shift.Cancelled, ChangeReason: shift.ChangeReason, OriginShiftID: shift.OriginShiftID,
		SickAbsenceID: shift.SickAbsenceID, CreatedBy: shift.CreatedBy, UpdatedBy: shift.UpdatedBy,
		CreatedAt: shift.CreatedAt, UpdatedAt: shift.UpdatedAt,
	}
	if shift.SeriesOccurrenceDate != nil {
		value.SeriesOccurrenceDate = shift.SeriesOccurrenceDate.String()
	}
	return value
}

func shiftReadRecord(value domain.StaffShift) *workforce.ShiftRecord {
	shift := &workforce.ShiftRecord{}
	shift.ID = value.ID
	shift.CreatedAt = value.CreatedAt
	shift.UpdatedAt = value.UpdatedAt
	shift.TenantID = value.TenantID
	shift.StaffID = value.StaffID
	shift.Date = calendar.Date(value.Date)
	shift.StartTime = shiftReadClock(value.StartTime)
	shift.EndTime = shiftReadClock(value.EndTime)
	shift.BreakMinutes = value.BreakMinutes
	shift.ShiftTypeID = value.ShiftTypeID
	shift.Notes = value.Notes
	shift.SeriesID = value.SeriesID
	shift.Detached = value.Detached
	shift.SeriesOccurrenceDate = nil
	if value.SeriesOccurrenceDate != "" {
		occurrence := calendar.Date(value.SeriesOccurrenceDate)
		shift.SeriesOccurrenceDate = &occurrence
	}
	shift.Cancelled = value.Cancelled
	shift.ChangeReason = value.ChangeReason
	shift.OriginShiftID = value.OriginShiftID
	shift.SickAbsenceID = value.SickAbsenceID
	shift.CreatedBy = value.CreatedBy
	shift.UpdatedBy = value.UpdatedBy
	return shift
}

func shiftReadsToPublic(values []*workforce.ShiftRecord) []workforce.StaffShift {
	result := make([]workforce.StaffShift, 0, len(values))
	for _, value := range values {
		result = append(result, shiftReadToPublic(value))
	}
	return result
}

func shiftReadClock(value string) time.Time {
	parsed, err := time.Parse(domain.ClockLayout, value)
	if err != nil {
		return time.Time{}
	}
	return time.Date(1, time.January, 1, parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.UTC)
}
