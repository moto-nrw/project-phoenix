package api

import (
	"context"
	"errors"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The bindings below connect the absence list of moto schule (#4793) to
// Student Presence's morning marks, Care Plan's status days, the School
// Calendar's half-years and holidays, the GDPR access log and Document
// Rendering.

// classDayAbsenceStatusDayReader reads Care Plan's status days of many
// students in a period, the ones archived at the end of their day included.
type classDayAbsenceStatusDayReader interface {
	FindStandingByStudentIDsAndDateRange(ctx context.Context, studentIDs []int64, from, to absencerecords.Date) ([]*absencerecords.StudentStatusDay, error)
}

// classDayMorningMarkReader reads Student Presence's morning marks of many
// children in a period.
type classDayMorningMarkReader interface {
	MorningAttendanceBetween(ctx context.Context, from, to timezone.Date, children []studentpresence.MorningAttendanceChild) ([]studentpresence.MorningAttendanceEntry, error)
}

// withClassDayAbsenceList binds the absence list's ports into the morning
// attendance seam, whose marks it counts.
func withClassDayAbsenceList(
	morning classdayCompose.MorningAttendanceDependencies, marks classDayMorningMarkReader, statusDays classDayAbsenceStatusDayReader,
	schoolDays schoolcalendar.Days, accessLog owners.AccessLogLedger, renderer lists.Renderer,
	settings interface {
		ResolveInt(context.Context, string) (int, error)
	}, participation careplan.CareParticipation, reports enrollment.Reports,
) classdayCompose.MorningAttendanceDependencies {
	morning.AbsenceList.Participation = participation
	if reports != nil {
		morning.AbsenceList.Children = func(ctx context.Context, schoolClass string) ([]classday.AbsenceListChild, error) {
			rows, err := reports.ClassReportChildren(ctx, schoolClass)
			if err != nil {
				return nil, err
			}
			children := make([]classday.AbsenceListChild, 0, len(rows))
			for _, row := range rows {
				children = append(children, classday.AbsenceListChild{StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName})
			}
			return children, nil
		}
	}
	if settings != nil {
		morning.AbsenceList.AvailableFrom = classDayAbsenceSettings{settings: settings}.AvailableFrom
	}

	if marks != nil {
		morning.AbsenceList.Marks = classDayMorningMarks{marks: marks}
	}
	if statusDays != nil {
		morning.AbsenceList.StatusDays = classDayAbsenceStatusDays{reader: statusDays}
	}
	if schoolDays != nil {
		morning.AbsenceList.Calendar = classDayAbsenceCalendar{calendar: schoolDays}
	}
	if accessLog != nil {
		morning.AbsenceList.Log = classDayAccessLog{repo: accessLog}
	}
	if renderer != nil {
		morning.AbsenceList.Renderer = ClassDaySlotListRenderer{Renderer: renderer}
	}
	return morning
}

// classDayAbsenceSettings applies the shorter of the source retention windows.
type classDayAbsenceSettings struct {
	settings interface {
		ResolveInt(context.Context, string) (int, error)
	}
}

func (b classDayAbsenceSettings) AvailableFrom(ctx context.Context, today timezone.Date) (timezone.Date, error) {
	marksDays, err := b.settings.ResolveInt(ctx, configModels.KeyGDPRMorningAttendanceRetentionDays)
	if err != nil {
		return "", err
	}
	statusDays, err := b.settings.ResolveInt(ctx, configModels.KeyGDPRStatusDayRetentionDays)
	if err != nil {
		return "", err
	}
	return today.AddDays(-min(marksDays, statusDays)), nil
}

// classDayMorningMarks reads Student Presence's morning marks of a period.
type classDayMorningMarks struct{ marks classDayMorningMarkReader }

func (b classDayMorningMarks) MorningMarksBetween(ctx context.Context, from, to timezone.Date, studentIDs, listEntryIDs []int64) ([]classday.MorningMark, error) {
	children := make([]studentpresence.MorningAttendanceChild, 0, len(studentIDs)+len(listEntryIDs))
	for _, id := range studentIDs {
		children = append(children, studentpresence.MorningAttendanceChild{StudentID: id})
	}
	for _, id := range listEntryIDs {
		children = append(children, studentpresence.MorningAttendanceChild{ClassListEntryID: id})
	}
	entries, err := b.marks.MorningAttendanceBetween(ctx, from, to, children)
	if err != nil {
		return nil, err
	}
	out := make([]classday.MorningMark, 0, len(entries))
	for _, entry := range entries {
		out = append(out, classday.MorningMark{
			StudentID: entry.Child.StudentID, ListEntryID: entry.Child.ClassListEntryID, Date: entry.Date, Status: entry.Status,
		})
	}
	return out, nil
}

// classDayAbsenceStatusDays reads Care Plan's status days of a period.
type classDayAbsenceStatusDays struct {
	reader classDayAbsenceStatusDayReader
}

func (b classDayAbsenceStatusDays) StatusDaysBetween(ctx context.Context, studentIDs []int64, from, to timezone.Date) ([]classday.DatedStatusDay, error) {
	if len(studentIDs) == 0 {
		return []classday.DatedStatusDay{}, nil
	}
	rows, err := b.reader.FindStandingByStudentIDsAndDateRange(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	out := make([]classday.DatedStatusDay, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			out = append(out, classday.DatedStatusDay{StudentID: row.StudentID, Date: row.Date, Status: row.Status})
		}
	}
	return out, nil
}

// classDayAbsenceCalendar reads the School Calendar's half-years, school
// holidays and statutory holidays.
type classDayAbsenceCalendar struct{ calendar schoolcalendar.Days }

func (b classDayAbsenceCalendar) CurrentHalfYear(ctx context.Context, day timezone.Date) (classday.HalfYear, bool, error) {
	periods, err := b.calendar.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{
		PeriodType: schoolcalendar.PeriodTypeSemester, ActiveOnly: true, OverlappingFrom: day.String(), OverlappingTo: day.String(),
	})
	if err != nil {
		return classday.HalfYear{}, false, err
	}
	for _, period := range periods {
		from, fromErr := timezone.ParseDate(period.StartDate)
		to, toErr := timezone.ParseDate(period.EndDate)
		if fromErr == nil && toErr == nil {
			return classday.HalfYear{Name: period.Name, From: from, To: to}, true, nil
		}
	}
	return classday.HalfYear{}, false, nil
}

// SchoolFreeDays joins the active holiday periods (Ferien) and the statutory
// holidays of the school's federal state. A school without a federal state
// counts no statutory holiday instead of failing the list.
func (b classDayAbsenceCalendar) SchoolFreeDays(ctx context.Context, from, to timezone.Date) (map[timezone.Date]bool, error) {
	free := map[timezone.Date]bool{}
	holidays, err := b.calendar.TenantHolidayDates(ctx, from.String(), to.String())
	if err != nil && !errors.Is(err, schoolcalendar.ErrFederalStateUnavailable) {
		return nil, err
	}
	for day, closed := range holidays {
		if closed {
			free[timezone.Date(day)] = true
		}
	}
	periods, err := b.calendar.ListCalendarPeriods(ctx, schoolcalendar.HolidaysOverlapping(from.String(), to.String()))
	if err != nil {
		return nil, err
	}
	for _, period := range periods {
		addHolidayPeriodDays(free, period, from, to)
	}
	return free, nil
}

// addHolidayPeriodDays marks the days of one holiday period inside the
// window; a period with a broken date marks nothing.
func addHolidayPeriodDays(free map[timezone.Date]bool, period schoolcalendar.CalendarPeriod, from, to timezone.Date) {
	start, startErr := timezone.ParseDate(period.StartDate)
	end, endErr := timezone.ParseDate(period.EndDate)
	if startErr != nil || endErr != nil {
		return
	}
	if start.Before(from) {
		start = from
	}
	if end.After(to) {
		end = to
	}
	for day := start; !day.After(end); day = day.AddDays(1) {
		free[day] = true
	}
}

func (l classDayAccessLog) RecordAbsenceList(ctx context.Context, entry classdayCompose.AccessRecord) error {
	return l.repo.Create(ctx, classDayAccessRow(auditModels.ResourceTypeClassAbsenceList, entry))
}

func (l classDayAccessLog) RecordAbsenceListExport(ctx context.Context, entry classdayCompose.AccessRecord) error {
	return l.repo.Create(ctx, classDayAccessRow(auditModels.ResourceTypeClassAbsenceListExport, entry))
}
