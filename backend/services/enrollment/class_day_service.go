package enrollment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// StudentStatusDayCancelled is the class-day-only marker for a pickup
// exception without a time: the care for this day was called off ("kommt
// heute nicht"). Not a persisted active.student_status_days status.
const StudentStatusDayCancelled = "cancelled"

// ClassDayRow is one student of the class on the requested day, reduced to
// what the handoff after lessons needs (#1772). Deliberately NO guardian
// names or contact details: the Lehrkraft view is a privacy-reduced
// projection of the class roster — who stays, who goes home, and how.
type ClassDayRow struct {
	StudentID  int64    `json:"student_id"`
	FirstName  string   `json:"first_name"`
	LastName   string   `json:"last_name"`
	GroupName  string   `json:"group_name,omitempty"`
	Registered bool     `json:"registered"`
	StaysToday bool     `json:"stays_today"`
	Offerings  []string `json:"offerings"`
	Arrival    string   `json:"arrival,omitempty"`
	Pickup     string   `json:"pickup,omitempty"`
	Departure  string   `json:"departure,omitempty"`
	// Status is the scheduled day status ("sick" / "excused" / "class_trip",
	// plus the derived "cancelled" when a pickup exception calls the care day
	// off), empty when none is reported. The free-text note stays private.
	Status string `json:"status,omitempty"`
}

type ClassDayTotals struct {
	Students int `json:"students"`
	Staying  int `json:"staying"`
	Leaving  int `json:"leaving"`
	Absent   int `json:"absent"`
}

type ClassDayReport struct {
	SchoolClass string        `json:"school_class"`
	Date        timezone.Date `json:"date"`
	// Weekday is the report day key ("mon".."fri"), empty on weekends.
	Weekday   string         `json:"weekday"`
	SchoolDay bool           `json:"school_day"`
	PhaseName string         `json:"phase_name,omitempty"`
	Totals    ClassDayTotals `json:"totals"`
	Rows      []ClassDayRow  `json:"rows"`
}

// classDayWeekdayKey maps a calendar date onto the report day keys used by
// OfferingsByDay / PickupByDay ("mon".."fri"). Weekend dates return "".
func classDayWeekdayKey(date timezone.Date) string {
	switch date.Weekday() {
	case time.Monday:
		return "mon"
	case time.Tuesday:
		return "tue"
	case time.Wednesday:
		return "wed"
	case time.Thursday:
		return "thu"
	case time.Friday:
		return "fri"
	default:
		return ""
	}
}

// classDayPhase resolves the enrollment phase whose service window covers the
// date. Prefers an active covering phase, then any covering phase with the
// latest start. Returns nil when no phase covers the date — the class day
// view then still lists the class, just without enrollment data.
func (s *reportService) classDayPhase(ctx context.Context, date timezone.Date) (phaseID int64, phaseName string, err error) {
	phases, err := s.PhaseRepo.ListByTenant(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("class day report: list phases: %w", err)
	}
	var best *struct {
		id     int64
		name   string
		start  timezone.Date
		active bool
	}
	for _, phase := range phases {
		if phase == nil {
			continue
		}
		if date.Before(phase.ServiceStartDate) || date.After(phase.ServiceEndDate) {
			continue
		}
		candidate := &struct {
			id     int64
			name   string
			start  timezone.Date
			active bool
		}{id: phase.ID, name: phase.Name, start: phase.ServiceStartDate, active: phase.IsActive}
		if best == nil ||
			(candidate.active && !best.active) ||
			(candidate.active == best.active && best.start.Before(candidate.start)) {
			best = candidate
		}
	}
	if best == nil {
		return 0, "", nil
	}
	return best.id, best.name, nil
}

// classDayStatuses loads the scheduled day statuses of the listed students.
// Precedence mirrors the dashboard counts: sick wins, class trip beats a
// plain excuse.
func (s *reportService) classDayStatuses(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]string, error) {
	out := make(map[int64]string, len(studentIDs))
	if s.StudentStatusDayRepo == nil || len(studentIDs) == 0 {
		return out, nil
	}
	entries, err := s.StudentStatusDayRepo.FindActiveByStudentIDsAndDate(ctx, studentIDs, date)
	if err != nil {
		return nil, fmt.Errorf("class day report: load status days: %w", err)
	}
	rank := map[string]int{
		activeModels.StudentStatusDaySick:      3,
		activeModels.StudentStatusDayClassTrip: 2,
		activeModels.StudentStatusDayExcused:   1,
	}
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		if rank[entry.Status] > rank[out[entry.StudentID]] {
			out[entry.StudentID] = entry.Status
		}
	}
	return out, nil
}

// classDayRosterRows builds class-roster rows for the class without an
// enrollment phase: full class list, group names, and the departure plan
// stored on the student — every "Keine Anmeldung" default the roster row
// builder produces for a nil enrollment.
func (s *reportService) classDayRosterRows(ctx context.Context, schoolClass string) ([]ClassRosterRow, error) {
	if s.StudentRepo == nil || s.PersonRepo == nil || s.EducationGroupRepo == nil {
		return nil, fmt.Errorf("class day report: repos not configured")
	}
	students, err := s.classRosterStudents(ctx, ClassRosterFilters{SchoolClass: schoolClass})
	if err != nil {
		return nil, err
	}
	if len(students) > maxReportRows {
		return nil, fmt.Errorf("class day report: %d students: %w", len(students), ErrReportExportTooLarge)
	}
	persons, err := s.PersonRepo.FindByIDs(ctx, classRosterPersonIDs(students))
	if err != nil {
		return nil, fmt.Errorf("class day report: load persons: %w", err)
	}
	groups, err := s.classRosterGroupNames(ctx, students)
	if err != nil {
		return nil, err
	}
	companions, err := s.classRosterCompanions(ctx, classRosterStudentIDs(students))
	if err != nil {
		return nil, err
	}
	rows := make([]ClassRosterRow, 0, len(students))
	for _, student := range students {
		if student == nil {
			continue
		}
		row, err := classRosterRow(student, persons[student.PersonID], classRosterGroupName(student, groups), nil, nil, nil, nil, companions[student.ID])
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	sortClassRosterRows(rows)
	return rows, nil
}

// ClassDay builds the read-only per-class day view for the Lehrkraft handoff
// (#1772): the full class (including students without any enrollment), per
// student whether they stay in care on that day, which offerings apply, the
// arrival/pickup times, and the departure plan. Caller scoping (which classes
// the account may see) is the API layer's job via usercontext — this method
// only knows the class name.
func (s *reportService) ClassDay(ctx context.Context, schoolClass string, date timezone.Date, actorAccountID int64) (*ClassDayReport, error) {
	schoolClass = strings.TrimSpace(schoolClass)
	if schoolClass == "" {
		return nil, fmt.Errorf("class day report: school class required: %w", ErrReportInvalidFilter)
	}

	phaseID, phaseName, err := s.classDayPhase(ctx, date)
	if err != nil {
		return nil, err
	}

	var rosterRows []ClassRosterRow
	if phaseID != 0 {
		roster, err := s.ClassRoster(ctx, ClassRosterFilters{PhaseID: phaseID, SchoolClass: schoolClass})
		if err != nil {
			return nil, err
		}
		rosterRows = roster.Rows
	} else {
		rosterRows, err = s.classDayRosterRows(ctx, schoolClass)
		if err != nil {
			return nil, err
		}
	}

	studentIDs := make([]int64, 0, len(rosterRows))
	for _, row := range rosterRows {
		studentIDs = append(studentIDs, row.StudentID)
	}
	statuses, err := s.classDayStatuses(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	departures, err := s.classDayDepartures(ctx, schoolClass, studentIDs, classDayWeekdayKey(date))
	if err != nil {
		return nil, err
	}
	arrivals, pickups, cancelled, err := s.classDayEffectiveTimes(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	for studentID := range cancelled {
		// A stronger reported status (sick / class trip / excused) keeps
		// precedence over the plain "kommt heute nicht" cancellation.
		if statuses[studentID] == "" {
			statuses[studentID] = StudentStatusDayCancelled
		}
	}

	report := buildClassDayReport(schoolClass, date, phaseName, rosterRows, statuses, departures, arrivals, pickups)

	if err := s.recordClassDayViewAudit(ctx, report, actorAccountID); err != nil {
		return nil, err
	}
	return report, nil
}

// classDayModeLabels are the day-view labels for a single day's departure
// modes; unlike the roster's week summary there is no day prefix.
var classDayModeLabels = map[userModels.DepartureMode]string{
	userModels.DepartureAlone:       "Geht alleine",
	userModels.DepartureBus:         "Bus",
	userModels.DeparturePickup:      "Abholung",
	userModels.DepartureAccompanied: "Mit anderem Kind",
}

// classDayEffectiveTimes loads the effective arrival/pickup times of the
// students for the date from the materialized schedule tables (weekly plan
// plus day exceptions) — the current truth the roster's form-answer snapshot
// may lag behind. A pickup exception WITHOUT a time cancels the care day.
func (s *reportService) classDayEffectiveTimes(ctx context.Context, studentIDs []int64, date timezone.Date) (arrivals, pickups map[int64]string, cancelled map[int64]bool, err error) {
	arrivals = map[int64]string{}
	pickups = map[int64]string{}
	cancelled = map[int64]bool{}
	if len(studentIDs) == 0 {
		return arrivals, pickups, cancelled, nil
	}
	if s.PickupScheduleSvc != nil {
		effective, err := s.PickupScheduleSvc.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, date)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("class day report: load effective pickup times: %w", err)
		}
		for studentID, entry := range effective {
			if entry == nil {
				continue
			}
			switch {
			case entry.PickupTime != nil:
				pickups[studentID] = entry.PickupTime.Format("15:04")
			case entry.IsException:
				cancelled[studentID] = true
			}
		}
	}
	if s.ArrivalScheduleSvc != nil {
		effective, err := s.ArrivalScheduleSvc.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, date)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("class day report: load effective arrival times: %w", err)
		}
		for studentID, entry := range effective {
			if entry != nil && entry.ArrivalTime != nil {
				arrivals[studentID] = entry.ArrivalTime.Format("15:04")
			}
		}
	}
	return arrivals, pickups, cancelled, nil
}

// classDayDepartures renders the departure plan of every student REDUCED to
// the requested weekday ("Abholung" instead of "Mo: Abholung, Di: …"): the
// handoff sheet answers today, not the week. Source is the live student row
// (AllowedDepartureModes with the DepartureDays fallback) — the current
// truth, present for every child, enrolled or not. Companion names are
// attached for days that allow the accompanied mode. Empty map on weekends.
func (s *reportService) classDayDepartures(ctx context.Context, schoolClass string, studentIDs []int64, weekday string) (map[int64]string, error) {
	out := make(map[int64]string, len(studentIDs))
	if weekday == "" || len(studentIDs) == 0 || s.StudentRepo == nil {
		return out, nil
	}
	students, err := s.StudentRepo.FindBySchoolClass(ctx, schoolClass)
	if err != nil {
		return nil, fmt.Errorf("class day report: load departure plans: %w", err)
	}
	companions, err := s.classRosterCompanions(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	for _, student := range students {
		if student == nil {
			continue
		}
		out[student.ID] = classDayDeparture(student, weekday, companions[student.ID])
	}
	return out, nil
}

// classDayDeparture renders one student's departure for one weekday.
func classDayDeparture(student *userModels.Student, weekday string, companions []userModels.CompanionLink) string {
	allowed := student.AllowedDepartureModes.Normalize()
	if !allowed.HasAny() {
		allowed = userModels.AllowedDepartureModesFromDeparture(student.DepartureDays)
	}
	modes := allowed[weekday]
	if len(modes) == 0 {
		return "Geht alleine"
	}
	labels := make([]string, 0, len(modes))
	accompanied := false
	for _, mode := range modes {
		if mode == userModels.DepartureAccompanied {
			accompanied = true
		}
		if label := classDayModeLabels[mode]; label != "" {
			labels = append(labels, label)
		}
	}
	summary := strings.Join(labels, ", ")
	if accompanied {
		onDay := userModels.FilterCompanionLinksToDays(companions, map[string]bool{weekday: true})
		if linked := userModels.FormatCompanionLinks(onDay); linked != "" {
			summary += " (" + linked + ")"
		} else if note := student.DepartureCompanionNote; note != nil && strings.TrimSpace(*note) != "" {
			summary += " (" + strings.TrimSpace(*note) + ")"
		}
	}
	return summary
}

// buildClassDayReport projects full roster rows onto one calendar day: the
// weekday's offerings decide who stays, a reported day status wins over any
// enrollment, and everyone else goes home after lessons. Day-specific
// departures and effective arrival/pickup times (from the live plans)
// replace the roster's form-answer values when available.
func buildClassDayReport(schoolClass string, date timezone.Date, phaseName string, rosterRows []ClassRosterRow, statuses map[int64]string, departures, arrivals, pickups map[int64]string) *ClassDayReport {
	weekday := classDayWeekdayKey(date)
	report := &ClassDayReport{
		SchoolClass: schoolClass,
		Date:        date,
		Weekday:     weekday,
		SchoolDay:   weekday != "",
		PhaseName:   phaseName,
		Rows:        make([]ClassDayRow, 0, len(rosterRows)),
	}
	for _, row := range rosterRows {
		offerings := []string{}
		arrival, pickup := "", ""
		if weekday != "" {
			offerings = append(offerings, row.OfferingsByDay[weekday]...)
			arrival = strings.TrimSpace(row.ArrivalByDay[weekday])
			pickup = strings.TrimSpace(row.PickupByDay[weekday])
			if effective := arrivals[row.StudentID]; effective != "" {
				arrival = effective
			}
			if effective := pickups[row.StudentID]; effective != "" {
				pickup = effective
			}
		}
		status := statuses[row.StudentID]
		stays := len(offerings) > 0 && status == ""
		departure := row.Departure
		if dayDeparture, ok := departures[row.StudentID]; ok && dayDeparture != "" {
			departure = dayDeparture
		}
		dayRow := ClassDayRow{
			StudentID:  row.StudentID,
			FirstName:  row.FirstName,
			LastName:   row.LastName,
			GroupName:  row.GroupName,
			Registered: row.Registered,
			StaysToday: stays,
			Offerings:  offerings,
			Arrival:    arrival,
			Pickup:     pickup,
			Departure:  departure,
			Status:     status,
		}
		report.Rows = append(report.Rows, dayRow)
		report.Totals.Students++
		switch {
		case status != "":
			report.Totals.Absent++
		case stays:
			report.Totals.Staying++
		default:
			report.Totals.Leaving++
		}
	}
	return report
}

// recordClassDayViewAudit writes the GDPR access log for a class day view.
// The view shows care and departure data of an entire class to an external
// teacher, so each read is logged like the roster export — but attributed to
// the authenticated account by the repo's actor resolution.
func (s *reportService) recordClassDayViewAudit(ctx context.Context, report *ClassDayReport, actorAccountID int64) error {
	if s.DataAccessLogRepo == nil {
		return nil
	}
	entry, err := exportAuditEntry("class day view audit", actorAccountID, "lehrkraft",
		auditModels.ResourceTypeEnrollmentPhaseExport,
		report.Date.BerlinMidnight(), report.Date.EndOfDay(), time.Now())
	if err != nil {
		return err
	}
	entry.SetMetadata("report", "class_day")
	entry.SetMetadata("school_class", report.SchoolClass)
	entry.SetMetadata("date", report.Date.String())
	entry.SetMetadata("student_count", report.Totals.Students)
	return writeExportAudit(ctx, s.DataAccessLogRepo, entry, "class day view audit")
}
