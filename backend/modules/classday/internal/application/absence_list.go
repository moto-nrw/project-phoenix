package application

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/collation"
)

// Ports of the absence list (#4793). Student Presence stores the morning
// marks, Care Plan the status days, the School Calendar the half-years and
// the holidays; the root binds them.
type (
	// MorningMark, DatedStatusDay and HalfYear are the port values of the
	// public contract.
	MorningMark    = classday.MorningMark
	DatedStatusDay = classday.DatedStatusDay
	HalfYear       = classday.HalfYear
	// AbsenceListMarks reads the morning marks of many children in a period.
	AbsenceListMarks interface {
		MorningMarksBetween(ctx context.Context, from, to calendar.Date, studentIDs, listEntryIDs []int64) ([]MorningMark, error)
	}
	// AbsenceListStatusDays reads the status days of many students in a
	// period.
	AbsenceListStatusDays interface {
		StatusDaysBetween(ctx context.Context, studentIDs []int64, from, to calendar.Date) ([]DatedStatusDay, error)
	}
	// AbsenceListCalendar reads the School Calendar.
	AbsenceListCalendar interface {
		// CurrentHalfYear returns the half-year covering the day; found is
		// false when the school calendar has none.
		CurrentHalfYear(ctx context.Context, day calendar.Date) (period HalfYear, found bool, err error)
		// SchoolFreeDays returns the statutory holidays and the school
		// holidays (Ferien) from from to to.
		SchoolFreeDays(ctx context.Context, from, to calendar.Date) (map[calendar.Date]bool, error)
	}
	// AbsenceListLog appends the reads and exports of the absence list to
	// the GDPR access log.
	AbsenceListLog interface {
		RecordAbsenceList(ctx context.Context, entry AccessRecord) error
		RecordAbsenceListExport(ctx context.Context, entry AccessRecord) error
	}
)

// AbsenceListDependencies wire the absence list (#4793). Without Marks,
// StatusDays, Calendar, Children, AvailableFrom or Participation the list
// answers classday.ErrAbsenceListNotConfigured;
// without Log no access is recorded; without Renderer no file is rendered.
type AbsenceListDependencies struct {
	// AvailableFrom resolves the first day retained by both source windows.
	AvailableFrom func(context.Context, calendar.Date) (calendar.Date, error)
	Children      func(context.Context, string) ([]classday.AbsenceListChild, error)
	Participation interface {
		HistoricalParticipatingStudentIDsByDate(context.Context, []int64, calendar.Date, calendar.Date) (map[calendar.Date]map[int64]bool, error)
	}
	Marks      AbsenceListMarks
	StatusDays AbsenceListStatusDays
	Calendar   AbsenceListCalendar
	Log        AbsenceListLog
	Renderer   ListRenderer
}

// absenceList builds the absence list of a class (#4793). The class is
// resolved within the caller's reach before.
type absenceList struct {
	deps  AbsenceListDependencies
	today func() calendar.Date
}

// AbsenceListReport builds the list of one class and logs every read.
func (a absenceList) AbsenceListReport(ctx context.Context, query classday.AbsenceListQuery) (*classday.AbsenceListReport, error) {
	report, err := a.build(ctx, query)
	if err != nil {
		return nil, err
	}
	if err := a.recordView(ctx, report, query.Actor); err != nil {
		return nil, err
	}
	return report, nil
}

// RenderAbsenceList renders the list as a PDF or Excel file and logs every
// export.
func (a absenceList) RenderAbsenceList(ctx context.Context, query classday.AbsenceListQuery, format classday.Format) (classday.File, error) {
	if a.deps.Renderer == nil {
		return classday.File{}, classday.ErrAbsenceListNotConfigured
	}
	report, err := a.build(ctx, query)
	if err != nil {
		return classday.File{}, err
	}
	file, err := a.deps.Renderer.Render(absenceListDocument(report, time.Now()), format, absenceListFilename(report))
	if err != nil {
		return classday.File{}, err
	}
	entry, err := absenceListAccessRecord("absence list export audit", report, query.Actor)
	if err != nil {
		return classday.File{}, err
	}
	entry.Metadata["format"] = string(format)
	if a.deps.Log != nil {
		if err := a.deps.Log.RecordAbsenceListExport(ctx, entry); err != nil {
			return classday.File{}, fmt.Errorf("absence list export audit write: %w", err)
		}
	}
	return file, nil
}

func (a absenceList) build(ctx context.Context, query classday.AbsenceListQuery) (*classday.AbsenceListReport, error) {
	if a.deps.Marks == nil || a.deps.StatusDays == nil || a.deps.Calendar == nil || a.deps.AvailableFrom == nil || a.deps.Participation == nil || a.deps.Children == nil {
		return nil, classday.ErrAbsenceListNotConfigured
	}
	schoolClass := strings.TrimSpace(query.SchoolClass)
	if schoolClass == "" {
		return nil, fmt.Errorf("absence list: school class required: %w", errInvalidFilter)
	}
	today := a.today()
	period, err := a.period(ctx, query, today)
	if err != nil {
		return nil, err
	}
	availableFrom, err := a.deps.AvailableFrom(ctx, today)
	if err != nil {
		return nil, fmt.Errorf("absence list: retention window: %w", err)
	}
	report := &classday.AbsenceListReport{
		AvailableFrom: availableFrom,
		SchoolClass:   schoolClass, From: period.From, To: period.To,
		PeriodName: period.Name, Children: []classday.AbsenceListChild{},
	}
	// Only days up to today count: a day ahead has no mark yet.
	until := period.To
	if until.After(today) {
		until = today
	}
	ahead := until.Before(period.From)
	participationFrom, participationUntil := period.From, until
	if ahead {
		participationFrom, participationUntil = today, today
	}
	children, participation, err := a.children(ctx, schoolClass, participationFrom, participationUntil)
	if err != nil {
		return nil, err
	}

	if ahead {
		report.Children = children
		return report, nil
	}
	from := period.From
	if from.Before(availableFrom) {
		report.Incomplete = true
		from = availableFrom
	}
	if from.After(until) {
		report.Children = children
		return report, nil
	}
	report.CountedUntil = until
	if err := a.count(ctx, report, children, participation, from, until); err != nil {
		return nil, err
	}
	return report, nil
}

// children keeps every child who participated on any requested day.
func (a absenceList) children(ctx context.Context, schoolClass string, from, until calendar.Date) ([]classday.AbsenceListChild, map[calendar.Date]map[int64]bool, error) {
	children, err := a.deps.Children(ctx, schoolClass)
	if err != nil {
		return nil, nil, err
	}
	participationFrom, participationUntil := from, until
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.StudentID)
	}
	participation, err := a.deps.Participation.HistoricalParticipatingStudentIDsByDate(ctx, ids, participationFrom, participationUntil)
	if err != nil {
		return nil, nil, fmt.Errorf("absence list: participation: %w", err)
	}
	kept := make([]classday.AbsenceListChild, 0, len(children))
	for _, child := range children {
		for day := participationFrom; !day.After(participationUntil); day = day.AddDays(1) {
			if participation[day][child.StudentID] {
				kept = append(kept, child)
				break
			}
		}
	}
	children = kept
	sort.SliceStable(children, func(i, j int) bool {
		if children[i].LastName != children[j].LastName {
			return children[i].LastName < children[j].LastName
		}
		return children[i].FirstName < children[j].FirstName
	})
	return children, participation, nil
}

// count applies the retained window and each child's dated participation.
func (a absenceList) count(ctx context.Context, report *classday.AbsenceListReport, children []classday.AbsenceListChild, participation map[calendar.Date]map[int64]bool, from, until calendar.Date) error {
	schoolDays, err := a.schoolDays(ctx, from, until)
	if err != nil {
		return err
	}
	report.SchoolDays = len(schoolDays)
	days, err := a.childDays(ctx, children, from, until)
	if err != nil {
		return err
	}
	for i := range children {
		childID := children[i].StudentID
		childSchoolDays := make(map[calendar.Date]bool)
		for day := from; !day.After(until); day = day.AddDays(1) {
			if !participation[day][childID] {
				delete(days[childID], day)
			} else if schoolDays[day] {
				childSchoolDays[day] = true
			}
		}
		children[i].AbsenceListCounts = countAbsenceDays(days[childID], childSchoolDays)
		addAbsenceCounts(&report.Totals, children[i].AbsenceListCounts)
	}
	report.Children = children
	return nil
}

// period checks the requested period, or preselects the current half-year
// of the school calendar when none was requested.
// A chosen period carries no name.
func (a absenceList) period(ctx context.Context, query classday.AbsenceListQuery, today calendar.Date) (HalfYear, error) {
	if query.From.IsZero() && query.To.IsZero() {
		halfYear, found, err := a.deps.Calendar.CurrentHalfYear(ctx, today)
		if err != nil {
			return HalfYear{}, fmt.Errorf("absence list: current half-year: %w", err)
		}
		if !found {
			halfYear = fallbackHalfYear(today)
		}
		return halfYear, nil
	}
	from, to := query.From, query.To
	if from.IsZero() || to.IsZero() || to.Before(from) || from.AddDays(classday.MaxAbsenceListDays-1).Before(to) {
		return HalfYear{}, classday.ErrAbsenceListInvalidPeriod
	}
	return HalfYear{From: from, To: to}, nil
}

// fallbackHalfYear is the half-year of the German school year without a
// calendar entry: 1 August to 31 January, 1 February to 31 July. It carries
// no name, so nobody reads it as the school's own entry. The bounds follow
// schoolcalendar.DefaultSchoolYear.
func fallbackHalfYear(today calendar.Date) HalfYear {
	day := today.BerlinMidnight()
	year := day.Year()
	switch {
	case day.Month() >= time.August:
		return HalfYear{From: calendar.NewDate(year, time.August, 1), To: calendar.NewDate(year+1, time.January, 31)}
	case day.Month() == time.January:
		return HalfYear{From: calendar.NewDate(year-1, time.August, 1), To: calendar.NewDate(year, time.January, 31)}
	default:
		return HalfYear{From: calendar.NewDate(year, time.February, 1), To: calendar.NewDate(year, time.July, 31)}
	}
}

// schoolDays returns the school days from from to until: Monday to Friday
// without statutory holidays and school holidays. A weekend the OGS runs on
// Friday's plan stays school-free: that setting is about the OGS.
func (a absenceList) schoolDays(ctx context.Context, from, until calendar.Date) (map[calendar.Date]bool, error) {
	free, err := a.deps.Calendar.SchoolFreeDays(ctx, from, until)
	if err != nil {
		return nil, fmt.Errorf("absence list: school-free days: %w", err)
	}
	days := map[calendar.Date]bool{}
	for day := from; !day.After(until); day = day.AddDays(1) {
		if !calendar.IsWeekend(day) && !free[day] {
			days[day] = true
		}
	}
	return days, nil
}

// absenceDay is what one child's day holds: the morning mark, if any, and the
// strongest status day.
type absenceDay struct {
	mark   string
	status string
}

// childDays reads the morning marks and the status days of the children in
// one read each and folds them per child and day.
func (a absenceList) childDays(ctx context.Context, children []classday.AbsenceListChild, from, until calendar.Date) (map[int64]map[calendar.Date]absenceDay, error) {
	days := make(map[int64]map[calendar.Date]absenceDay, len(children))
	if len(children) == 0 {
		return days, nil
	}
	studentIDs := make([]int64, 0, len(children))
	for _, child := range children {
		studentIDs = append(studentIDs, child.StudentID)
		days[child.StudentID] = map[calendar.Date]absenceDay{}
	}
	marks, err := a.deps.Marks.MorningMarksBetween(ctx, from, until, studentIDs, nil)
	if err != nil {
		return nil, fmt.Errorf("absence list: load morning marks: %w", err)
	}
	for _, mark := range marks {
		if byDay, ok := days[mark.StudentID]; ok {
			day := byDay[mark.Date]
			day.mark = mark.Status
			byDay[mark.Date] = day
		}
	}
	statuses, err := a.deps.StatusDays.StatusDaysBetween(ctx, studentIDs, from, until)
	if err != nil {
		return nil, fmt.Errorf("absence list: load status days: %w", err)
	}
	for _, status := range statuses {
		if byDay, ok := days[status.StudentID]; ok {
			day := byDay[status.Date]
			day.status = strongerStatus(day.status, status.Status)
			byDay[status.Date] = day
		}
	}
	return days, nil
}

// strongerStatus keeps a class trip over sick or excused: the class was
// away, so the day is no absence day whatever else was entered.
func strongerStatus(current, next string) string {
	switch {
	case current == statusDayClassTrip || next == statusDayClassTrip:
		return statusDayClassTrip
	case excusesAbsence(next):
		return next
	default:
		return current
	}
}

// excusesAbsence reports a status day that excuses an absence day.
func excusesAbsence(status string) bool {
	return status == statusDaySick || status == statusDayExcused
}

// countAbsenceDays applies the counting rules of classday.AbsenceList to the
// days of one child. A day with a morning mark counts even when the calendar
// calls it school-free: the Lehrkraft recorded a school morning.
func countAbsenceDays(days map[calendar.Date]absenceDay, schoolDays map[calendar.Date]bool) classday.AbsenceListCounts {
	var counts classday.AbsenceListCounts
	for date, day := range days {
		switch {
		case day.mark == classday.MorningAttendancePresent, day.status == statusDayClassTrip:
		case day.mark == classday.MorningAttendanceAbsent:
			counts.AbsentDays++
			if excusesAbsence(day.status) {
				counts.ExcusedDays++
			} else {
				counts.UnexcusedDays++
			}
		case schoolDays[date] && excusesAbsence(day.status):
			counts.AbsentDays++
			counts.ExcusedDays++
		}
	}
	for date := range schoolDays {
		if day := days[date]; day.mark == "" && day.status == "" {
			counts.NotRecordedDays++
		}
	}
	return counts
}

func addAbsenceCounts(sum *classday.AbsenceListCounts, add classday.AbsenceListCounts) {
	sum.AbsentDays += add.AbsentDays
	sum.ExcusedDays += add.ExcusedDays
	sum.UnexcusedDays += add.UnexcusedDays
	sum.NotRecordedDays += add.NotRecordedDays
}

// recordView writes the GDPR access log of a read: one row per read. The
// page loads the list only when the class or the period changes, not on
// focus or on a timer, so every row is a deliberate look at the list.
func (a absenceList) recordView(ctx context.Context, report *classday.AbsenceListReport, actor classday.Actor) error {
	if a.deps.Log == nil {
		return nil
	}
	const errPrefix = "absence list audit"
	entry, err := absenceListAccessRecord(errPrefix, report, actor)
	if err != nil {
		return err
	}
	if err := a.deps.Log.RecordAbsenceList(ctx, entry); err != nil {
		return fmt.Errorf("%s write: %w", errPrefix, err)
	}
	return nil
}

func absenceListAccessRecord(errPrefix string, report *classday.AbsenceListReport, actor classday.Actor) (AccessRecord, error) {
	entry, err := newAccessRecord(errPrefix, actor.AccountID, actor.Roles, report.From.BerlinMidnight(), report.To.EndOfDay(), time.Now())
	if err != nil {
		return AccessRecord{}, err
	}
	entry.Metadata = map[string]any{
		"report":        "absence_list",
		"school_class":  report.SchoolClass,
		"from":          report.From.String(),
		"to":            report.To.String(),
		"student_count": len(report.Children),
	}
	return entry, nil
}

const exportDateLayout = "02.01.2006"

// absenceListDocument prints the list: one row per child, the class in the
// last row, the counting rules in the filters.
func absenceListDocument(report *classday.AbsenceListReport, generatedAt time.Time) classday.ExportDocument {
	subtitle := report.From.Format(exportDateLayout) + " bis " + report.To.Format(exportDateLayout)
	if report.PeriodName != "" {
		subtitle = report.PeriodName + " · " + subtitle
	}
	counted := "Noch kein Tag gezählt: Der Zeitraum liegt in der Zukunft."
	if !report.CountedUntil.IsZero() {
		counted = fmt.Sprintf("Gezählt bis %s · %d Schultage", report.CountedUntil.Format(exportDateLayout), report.SchoolDays)
	}
	if report.Incomplete {
		counted = "Unvollständiger Zeitraum. Gezählt wird erst ab " + report.AvailableFrom.Format(exportDateLayout) + ". Ältere Anwesenheit oder Meldungen können bereits gelöscht sein."
	}
	doc := classday.ExportDocument{
		Title:       "Fehlzeiten der " + collation.SchoolClassLabel(report.SchoolClass),
		Subtitle:    subtitle,
		GeneratedAt: generatedAt,
		Filters: []string{
			counted,
			"Entschuldigt: krank oder entschuldigt gemeldet. Klassenfahrt ist kein Fehltag.",
			"Nicht erfasst: Schultag ohne Anwesenheit und ohne Meldung. Das ist kein Fehltag.",
		},
		Columns: []classday.ExportColumn{
			{ID: classday.ExportColumnName, Label: "Name"},
			{ID: classday.ExportColumnAbsentDays, Label: "Fehltage"},
			{ID: classday.ExportColumnExcusedDays, Label: "Davon entschuldigt"},
			{ID: classday.ExportColumnUnexcusedDays, Label: "Davon unentschuldigt"},
			{ID: classday.ExportColumnNotRecordedDays, Label: "Nicht erfasst"},
		},
		Rows:   make([]classday.ExportRow, 0, len(report.Children)+1),
		Footer: confidentialityNote,
	}
	for _, child := range report.Children {
		doc.Rows = append(doc.Rows, absenceListExportRow(child.LastName+", "+child.FirstName, child.AbsenceListCounts))
	}
	doc.Rows = append(doc.Rows, absenceListExportRow("Klasse gesamt", report.Totals))
	return doc
}

func absenceListExportRow(name string, counts classday.AbsenceListCounts) classday.ExportRow {
	return classday.ExportRow{Values: map[classday.ExportColumnID]string{
		classday.ExportColumnName:            name,
		classday.ExportColumnAbsentDays:      strconv.Itoa(counts.AbsentDays),
		classday.ExportColumnExcusedDays:     strconv.Itoa(counts.ExcusedDays),
		classday.ExportColumnUnexcusedDays:   strconv.Itoa(counts.UnexcusedDays),
		classday.ExportColumnNotRecordedDays: strconv.Itoa(counts.NotRecordedDays),
	}}
}

// absenceListFilename names the file after the class and the period; the
// renderer makes the name safe. Class names often carry "Klasse" already.
func absenceListFilename(report *classday.AbsenceListReport) string {
	return fmt.Sprintf("fehlzeiten-%s-%s-bis-%s", report.SchoolClass, report.From, report.To)
}
