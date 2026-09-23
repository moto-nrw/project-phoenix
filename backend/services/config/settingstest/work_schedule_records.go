package settingstest

import (
	"context"

	config "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The settings package still carries the contractual work-schedule rows and
// the work-time templates (#3207 moves them to Workforce). Until it does, the
// fakes that serve those rows to the composition adapters live here, so the
// adapter tests script schedules in plain values instead of ORM rows.

// ScheduleRow is one contractual target-minutes row of a staff member's
// schedule version. A zero ValidUntil means the version is still open.
type ScheduleRow struct {
	StaffID        int64
	ValidFrom      calendar.Date
	ValidUntil     calendar.Date
	RotationLength int
	WeekIndex      int
	DayOfWeek      int
	TargetMinutes  int
}

// ScheduleRecords serves the given rows to every date range, and reports
// schedule history for staff 7. It satisfies the schedule-target record
// contract of the services composition root.
type ScheduleRecords struct{ rows []*config.StaffWorkSchedule }

// Schedules returns the record source for the given rows.
func Schedules(rows ...ScheduleRow) ScheduleRecords {
	converted := make([]*config.StaffWorkSchedule, 0, len(rows))
	for _, row := range rows {
		entry := &config.StaffWorkSchedule{
			StaffID:        row.StaffID,
			ValidFrom:      config.CalendarDate(row.ValidFrom),
			RotationLength: row.RotationLength,
			WeekIndex:      row.WeekIndex,
			DayOfWeek:      row.DayOfWeek,
			TargetMinutes:  row.TargetMinutes,
		}
		if !row.ValidUntil.IsZero() {
			until := config.CalendarDate(row.ValidUntil)
			entry.ValidUntil = &until
		}
		converted = append(converted, entry)
	}
	return ScheduleRecords{rows: converted}
}

func (r ScheduleRecords) FindByStaffIDsValidInRange(context.Context, []int64, config.CalendarDate, config.CalendarDate) ([]*config.StaffWorkSchedule, error) {
	return r.rows, nil
}

func (r ScheduleRecords) HasScheduleHistory(context.Context, int64) (bool, error) {
	return true, nil
}

func (r ScheduleRecords) FindStaffIDsWithScheduleHistory(_ context.Context, staffIDs []int64) (map[int64]bool, error) {
	result := make(map[int64]bool, len(staffIDs))
	for _, id := range staffIDs {
		result[id] = true
	}
	return result, nil
}

// TemplateEntry is the target minutes of one rotation slot of a work-time
// template.
type TemplateEntry struct {
	WeekIndex     int
	DayOfWeek     int
	TargetMinutes int
}

// Template is a named work-time template with its rotation anchor.
type Template struct {
	ID             int64
	RotationLength int
	Anchor         calendar.Date
	Entries        []TemplateEntry
}

// TemplateRecords serves one template and the error a partial read returns. It
// satisfies the work-time-template record contract of the services composition
// root; FindByIDs answers with a missing row ahead of the template, the way a
// batched read reports one absent id.
type TemplateRecords struct {
	row *config.WorkTimeModel
	err error
}

// Templates returns the record source for one template and a read error.
func Templates(template Template, err error) TemplateRecords {
	row := &config.WorkTimeModel{
		ID:                 template.ID,
		RotationLength:     template.RotationLength,
		RotationAnchorDate: config.CalendarDate(template.Anchor),
	}
	for _, entry := range template.Entries {
		row.Entries = append(row.Entries, &config.WorkTimeModelEntry{
			WeekIndex: entry.WeekIndex, DayOfWeek: entry.DayOfWeek, TargetMinutes: entry.TargetMinutes,
		})
	}
	return TemplateRecords{row: row, err: err}
}

func (r TemplateRecords) FindByID(context.Context, int64) (*config.WorkTimeModel, error) {
	return r.row, r.err
}

func (r TemplateRecords) FindByIDs(context.Context, []int64) ([]*config.WorkTimeModel, error) {
	return []*config.WorkTimeModel{nil, r.row}, r.err
}
