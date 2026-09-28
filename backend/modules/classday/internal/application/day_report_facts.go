package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// classDayStatusRank orders the scheduled day statuses. The scale leaves rank
// 1 free for UNKNOWN statuses: the status set is an explicit extension point,
// and a value this map does not know still means "reported for the day".
// Dropping it silently would render a reported-absent child as "bleibt in
// der Betreuung" — the exact failure this view exists to prevent. Known
// statuses keep precedence over an unknown one; precedence mirrors the
// dashboard counts: sick wins, class trip beats a plain excuse.
var classDayStatusRank = map[string]int{
	statusDaySick:      6,
	statusDayClassTrip: 4,
	statusDayExcused:   2,
}

// The scheduled day statuses Care Plan records; the day view serves them as
// the row status.
const (
	statusDaySick      = "sick"
	statusDayClassTrip = "class_trip"
	statusDayExcused   = "excused"
)

func statusRank(status string) int {
	if status == "" {
		return 0
	}
	if known, ok := classDayStatusRank[status]; ok {
		return known
	}
	return 1
}

// classDayStatuses loads the scheduled day statuses of the listed students
// with the report time of the winning status.
func (s *dayReports) classDayStatuses(ctx context.Context, studentIDs []int64, date timezone.Date) (statuses map[int64]string, reportedAt map[int64]time.Time, err error) {
	out := make(map[int64]string, len(studentIDs))
	stamps := make(map[int64]time.Time, len(studentIDs))
	if s.statusDays == nil || len(studentIDs) == 0 {
		return out, stamps, nil
	}
	entries, err := s.statusDays.ActiveStatusDays(ctx, studentIDs, date)
	if err != nil {
		return nil, nil, fmt.Errorf("class day report: load status days: %w", err)
	}
	for _, entry := range entries {
		if entry.Status == "" || statusRank(entry.Status) <= statusRank(out[entry.StudentID]) {
			continue
		}
		out[entry.StudentID] = entry.Status
		// The stamp follows the winning status, not the newest row: it
		// answers "since when is THIS the situation" (#2294).
		if !entry.ReportedAt.IsZero() {
			stamps[entry.StudentID] = entry.ReportedAt
		} else {
			delete(stamps, entry.StudentID)
		}
	}
	return out, stamps, nil
}

// classDayEffectiveTimes loads the effective arrival/pickup times of the
// students for the date from the materialized schedule tables (weekly plan
// plus day exceptions) — the current truth the roster's form-answer snapshot
// may lag behind. Times only: the "kommt heute nicht" decision belongs to
// classDayCancellations.
func (s *dayReports) classDayEffectiveTimes(ctx context.Context, studentIDs []int64, date timezone.Date, facts *classDayFacts) error {
	if len(studentIDs) == 0 || s.times == nil {
		return nil
	}
	pickups, err := s.times.EffectivePickups(ctx, studentIDs, date)
	if err != nil {
		return fmt.Errorf("class day report: load effective pickup times: %w", err)
	}
	for studentID, entry := range pickups {
		applyClassDayPickup(facts, studentID, entry)
	}
	arrivals, err := s.times.EffectiveArrivals(ctx, studentIDs, date)
	if err != nil {
		return fmt.Errorf("class day report: load effective arrival times: %w", err)
	}
	for studentID, entry := range arrivals {
		applyClassDayArrival(facts, studentID, entry)
	}
	return nil
}

func applyClassDayArrival(facts *classDayFacts, studentID int64, entry EffectiveArrival) {
	if entry.ArrivalTime != nil {
		facts.arrivals[studentID] = entry.ArrivalTime.Format("15:04")
	}
	if entry.IsException && entry.ArrivalTime == nil && entry.ChangedAt != nil && !entry.ChangedAt.IsZero() {
		facts.arrivalCancelledAt[studentID] = *entry.ChangedAt
	}
}

// applyClassDayPickup records one student's pickup facts for the day: the
// effective time, and whether it deviates from the recurring plan.
//
// A day exception alone is NOT a deviation — a parent may re-enter the time
// the plan already holds, and announcing "geht heute um 15:00 statt 15:00"
// would train the Lehrkraft to ignore the block. The deviation is the
// comparison: a different clock time, or a time on a day the plan has none
// (the child normally is not in care then). A timeless exception carries no
// time at all; that is "kommt heute nicht" and travels as a status, not as a
// changed pickup.
func applyClassDayPickup(facts *classDayFacts, studentID int64, entry EffectivePickup) {
	if entry.PickupTime != nil {
		facts.pickups[studentID] = entry.PickupTime.Format("15:04")
	}
	if !entry.IsException {
		return
	}
	if entry.ChangedAt != nil && !entry.ChangedAt.IsZero() {
		facts.pickupChangedAt[studentID] = *entry.ChangedAt
	}
	if entry.PickupTime == nil {
		return
	}
	effective := entry.PickupTime.Format("15:04")
	regular := ""
	if entry.RegularPickupTime != nil {
		regular = entry.RegularPickupTime.Format("15:04")
	}
	if regular == effective {
		return
	}
	facts.pickupChanged[studentID] = true
	facts.pickupRegular[studentID] = regular
}

// classDayCancellations resolves which students are not coming that day,
// through Care Plan's care-day derivation: a timeless exception on EITHER leg
// (arrival or pickup) cancels the day, with the same precedence the student
// search, parent portal and Tagesauswertung use. Deliberately not derived
// from raw entries here. A not-scheduled care day ("an dem Tag nicht
// gebucht", e.g. the parents struck the weekday from the care plan while the
// approved offering still lists it) is recorded separately: not an absence,
// but the child must not be listed as staying either — every other reader
// treats it as not-expected.
func (s *dayReports) classDayCancellations(ctx context.Context, studentIDs []int64, date timezone.Date, facts *classDayFacts) (cancelled map[int64]bool, err error) {
	cancelled = map[int64]bool{}
	if s.careDays == nil || len(studentIDs) == 0 {
		return cancelled, nil
	}
	statuses, err := s.careDays.ResolveForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, fmt.Errorf("class day report: resolve care days: %w", err)
	}
	for studentID, status := range statuses {
		switch status {
		case ports.CareDayCancelled:
			cancelled[studentID] = true
		case ports.CareDayNotScheduled:
			facts.notScheduled[studentID] = true
		}
	}
	return cancelled, nil
}

// classDayDepartureUnknown renders a student without any departure data for
// the day. Deliberately distinct from an explicit "Geht alleine" plan: on
// the handoff sheet, missing data is a question for the office, not an
// instruction.
const classDayDepartureUnknown = "Keine Angabe"

// classDayModeLabels are the day-view labels for a single day's departure
// modes on the handoff sheet; the roster cells use their own lowercase
// phrasing.
var classDayModeLabels = map[peopledirectory.DepartureMode]string{
	peopledirectory.DepartureAlone:       "Geht alleine",
	peopledirectory.DepartureBus:         "Bus",
	peopledirectory.DeparturePickup:      "Abholung",
	peopledirectory.DepartureAccompanied: "Mit anderem Kind",
}

// classDayDepartures renders the departure plan of every student REDUCED to
// the requested weekday ("Abholung" instead of "Mo: Abholung, Di: …"): the
// handoff sheet answers today, not the week. Source is the live student plan
// Enrollment's day roster carries — the current truth, present for every
// child, enrolled or not. Companion names are attached only for children of
// this very class (see classDayDeparture). Empty map on weekends.
func (s *dayReports) classDayDepartures(ctx context.Context, students []DayRosterStudent, weekday string) (map[int64]string, error) {
	out := make(map[int64]string, len(students))
	if weekday == "" || len(students) == 0 {
		return out, nil
	}
	studentIDs := dayRosterStudentIDs(students)
	companions, err := s.companionLinks(ctx, studentIDs, "class roster report")
	if err != nil {
		return nil, err
	}
	// The class being served IS the disclosure boundary of this view, so it
	// is also the set of names a companion entry may mention.
	onSheet := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		onSheet[id] = true
	}
	for _, student := range students {
		out[student.ID] = classDayDeparture(student.DepartureModes, weekday, companions[student.ID], onSheet)
	}
	return out, nil
}

// dayRosterStudentIDs lists each positive student id once.
func dayRosterStudentIDs(students []DayRosterStudent) []int64 {
	ids := make([]int64, 0, len(students))
	seen := map[int64]bool{}
	for _, student := range students {
		if student.ID <= 0 || seen[student.ID] {
			continue
		}
		seen[student.ID] = true
		ids = append(ids, student.ID)
	}
	return ids
}

// companionLinks loads the "läuft mit" links of the students. Without the
// binding a departure names no companion.
func (s *dayReports) companionLinks(ctx context.Context, studentIDs []int64, errPrefix string) (map[int64][]peopledirectory.CompanionLink, error) {
	if s.companions == nil || len(studentIDs) == 0 {
		return map[int64][]peopledirectory.CompanionLink{}, nil
	}
	links, err := s.companions.ListLinksForStudents(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("%s: load departure companions: %w", errPrefix, err)
	}
	return links, nil
}

// classDayCompanionsOnSheet drops companion links pointing at children the
// caller does not already see on this sheet. A Laufgemeinschaft may pair a 1a
// child with a 4c child (users.student_companions is tenant-scoped, not
// class-scoped), and the class-day view is deliberately narrower than the
// tenant-wide student directory: a Lehrkraft holds class_day:read for their
// own education.class_teachers assignments, never users:read.
func classDayCompanionsOnSheet(links []peopledirectory.CompanionLink, onSheet map[int64]bool) []peopledirectory.CompanionLink {
	kept := make([]peopledirectory.CompanionLink, 0, len(links))
	for _, link := range links {
		if onSheet[link.CompanionStudentID] {
			kept = append(kept, link)
		}
	}
	return kept
}

// classDayDeparture renders one student's departure modes for one weekday.
// No plan data for the day means UNKNOWN, never "Geht alleine": on a sheet
// whose purpose is "wer geht wie nach Hause", missing data must not read as
// the instruction to let the child leave unaccompanied. The empty string
// makes buildClassDayReport render classDayDepartureUnknown — deliberately
// NOT the roster's form answer, whose per-day map is never empty and floors
// at "geht alleine" itself.
//
// onSheet holds the students of the class being served; an accompanied
// departure names only companions from that set. The free-text companion
// note is deliberately NOT a fallback here: parents write it, and "Abholung
// durch Nachbarin Frau Meier, 0171-…" is exactly the guardian name and
// contact detail this view promises never to show. An accompanied departure
// the sheet cannot name stays "Mit anderem Kind" — a question for the office,
// like every other gap on this sheet.
func classDayDeparture(modes []peopledirectory.DepartureMode, weekday string, companions []peopledirectory.CompanionLink, onSheet map[int64]bool) string {
	if len(modes) == 0 {
		return ""
	}
	labels := make([]string, 0, len(modes))
	accompanied := false
	for _, mode := range modes {
		if mode == peopledirectory.DepartureAccompanied {
			accompanied = true
		}
		if label := classDayModeLabels[mode]; label != "" {
			labels = append(labels, label)
		}
	}
	summary := strings.Join(labels, ", ")
	if accompanied {
		onDay := peopledirectory.FilterCompanionLinksToDays(companions, map[string]bool{weekday: true})
		if linked := peopledirectory.FormatCompanionLinks(classDayCompanionsOnSheet(onDay, onSheet)); linked != "" {
			summary += " (" + linked + ")"
		}
	}
	return summary
}

// weekdayDepartureModes reads a student's allowed departure modes on one
// weekday, falling back to the exclusive day plan.
func weekdayDepartureModes(allowed peopledirectory.AllowedDepartureModes, days peopledirectory.DepartureDays, weekday string) []peopledirectory.DepartureMode {
	allowed = allowed.Normalize()
	if !allowed.HasAny() {
		allowed = peopledirectory.AllowedDepartureModesFromDeparture(days)
	}
	return allowed[weekday]
}
