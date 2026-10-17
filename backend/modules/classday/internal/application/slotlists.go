// Package application builds the class-day read projection (#2701): the slot
// lists of a day and the school portal's per-class day view. It reads the
// public owner queries of Timetable & Activities (schedule.activity_instances,
// schedule.instance_students), Student Presence (active.visits,
// active.attendance), People Directory (users.students, users.persons), School
// Structure (education.groups), Facilities (rooms) and Care Plan (status days,
// pickup exceptions) directly, and reaches the retained schedule services,
// settings and identity only through the consumer-owned ports in
// modules/classday/internal/ports. It never writes.
package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// confidentialityNote matches the wording of the other printed exports.
const confidentialityNote = "Vertraulich, nur für berechtigte Personen. Nach Gebrauch sicher vernichten."

const timeLayout = "15:04"

// Roster substatus values of schedule.instance_students the lists render as a
// registered absence. They are the wire values the Timetable owner validates.
const (
	substatusSick      = "sick"
	substatusExcused   = "excused"
	substatusFieldTrip = "field_trip"
)

// TimetableReader is the block and roster read seam the lists read; the
// binding joins the Timetable plan with the Student Presence execution.
type TimetableReader = ports.TimetableReader

// PresenceReader is the Student Presence query seam the lists read.
type PresenceReader interface {
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
}

// CarePlanReader is the Care Plan query seam the lists read: the broad day
// statuses (sick / excused / class trip) that sign a child off for a whole
// date, including the scheduler's end-of-day archived rows, and the same-day
// pickup exceptions whose excused_from signs a child off partially.
type CarePlanReader interface {
	ListStudentStatusDays(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error)
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
}

// StudentReader is the People Directory student seam the lists read.
type StudentReader interface {
	ListEnrolledStudents(ctx context.Context) ([]peopledirectory.Student, error)
	ListStudentsByID(ctx context.Context, ids []int64) ([]peopledirectory.Student, error)
}

// PersonReader is the People Directory person seam the lists read.
type PersonReader interface {
	ListPersonsByID(ctx context.Context, ids []int64) ([]peopledirectory.Person, error)
}

// GroupReader is the School Structure seam the lists read.
type GroupReader interface {
	ListGroupsByID(ctx context.Context, ids []int64) ([]schoolstructure.Group, error)
}

// RoomReader is the Facilities seam the lists read.
type RoomReader interface {
	FindRoom(ctx context.Context, id int64) (facilities.Room, error)
}

// SlotListDependencies wires the narrow read seams the lists need.
type SlotListDependencies struct {
	Timetable       TimetableReader
	Presence        PresenceReader
	CarePlan        CarePlanReader
	Students        StudentReader
	Persons         PersonReader
	Groups          GroupReader
	Rooms           RoomReader
	CareDays        ports.CareDays
	EffectiveTimes  ports.EffectiveTimes
	PickupBaselines ports.PickupBaselines
	Rules           ports.Rules
	Settings        ports.Settings
	Access          ports.ReadAccess
	ListExport      ListRenderer
	// Now overrides the service clock. Leave nil in production (defaults to
	// time.Now); tests inject a fixed instant for a deterministic weekday.
	Now func() time.Time
}

type service struct {
	timetable       TimetableReader
	presence        PresenceReader
	carePlan        CarePlanReader
	students        StudentReader
	persons         PersonReader
	groups          GroupReader
	rooms           RoomReader
	careDays        ports.CareDays
	effectiveTimes  ports.EffectiveTimes
	pickupBaselines ports.PickupBaselines
	rules           ports.Rules
	settings        ports.Settings
	access          ports.ReadAccess
	listExport      ListRenderer
	// now is the clock the service reads "today" and "has this slot started
	// yet" from. Production leaves it nil (→ time.Now); tests inject a fixed
	// instant so the pickup suite is not at the mercy of the weekday CI runs on.
	now func() time.Time
}

// NewSlotLists creates the slot list builder. Settings is a required
// dependency: the Ganztag pickup-cohort cutoffs are tenant configuration
// resolved from the settings registry, so a service wired without it would
// silently serve authoritative cohorts using hardcoded defaults unrelated to
// the registered values (#1565 review pass 1). A nil Settings is a wiring bug,
// so fail fast at construction rather than degrade at request time.
func NewSlotLists(deps SlotListDependencies) classday.SlotLists {
	if deps.Settings == nil {
		panic("classday application.NewSlotLists: Settings dependency is required")
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return &service{
		timetable:       deps.Timetable,
		presence:        deps.Presence,
		carePlan:        deps.CarePlan,
		students:        deps.Students,
		persons:         deps.Persons,
		groups:          deps.Groups,
		rooms:           deps.Rooms,
		careDays:        deps.CareDays,
		effectiveTimes:  deps.EffectiveTimes,
		pickupBaselines: deps.PickupBaselines,
		rules:           deps.Rules,
		settings:        deps.Settings,
		access:          deps.Access,
		listExport:      deps.ListExport,
		now:             now,
	}
}

// currentTime returns the service's notion of "now" (real clock in production,
// an injected fixed instant in tests).
func (s *service) currentTime() time.Time { return s.now() }

// todayDate returns the current calendar day in Berlin, derived from the
// service clock so the date guards move with an injected test clock.
func (s *service) todayDate() calendar.Date { return calendar.DateFromTime(s.now()) }

func (s *service) configured() bool {
	return s.timetable != nil && s.presence != nil && s.carePlan != nil &&
		s.students != nil && s.persons != nil && s.groups != nil && s.rooms != nil &&
		s.careDays != nil && s.effectiveTimes != nil && s.pickupBaselines != nil &&
		s.rules != nil && s.access != nil && s.listExport != nil
}

func (s *service) requireTimetableEnabled(ctx context.Context) error {
	enabled, err := s.settings.TimetableEnabled(ctx)
	if err != nil {
		return fmt.Errorf("resolve timetable feature flag: %w", err)
	}
	if !enabled {
		return classday.ErrTimetableDisabled
	}
	return nil
}

// parseDate resolves the contract's calendar day. A malformed value is a
// caller error and is refused before any read.
func parseDate(value classday.Date) (calendar.Date, error) {
	date, err := calendar.ParseDate(string(value))
	if err != nil {
		return "", fmt.Errorf("invalid date %q: expected YYYY-MM-DD", value)
	}
	return date, nil
}

// slotInstance is one activity instance of the day with its calendar day and
// wall-clock window resolved from the owner's wire strings.
type slotInstance struct {
	ID            int64
	Title         string
	Date          calendar.Date
	StartTime     time.Time
	EndTime       time.Time
	RoomID        int64
	Status        string
	ActiveGroupID *int64
	ListKind      *string
}

// loadInstances reads every activity instance of the date in start-time
// order, exactly as every other reader of the day sees them.
func (s *service) loadInstances(ctx context.Context, date calendar.Date) ([]*slotInstance, error) {
	rows, err := s.timetable.ListActivityInstancesOn(ctx, date.String())
	if err != nil {
		return nil, err
	}
	instances := make([]*slotInstance, 0, len(rows))
	for _, row := range rows {
		instance, err := slotInstanceFromTimetable(row)
		if err != nil {
			return nil, err
		}
		instances = append(instances, instance)
	}
	return instances, nil
}

func slotInstanceFromTimetable(row ports.ActivityInstance) (*slotInstance, error) {
	date, err := calendar.ParseDate(row.Date)
	if err != nil {
		return nil, fmt.Errorf("activity instance %d: invalid date %q: %w", row.ID, row.Date, err)
	}
	start, err := parseWallClock(row.StartTime)
	if err != nil {
		return nil, fmt.Errorf("activity instance %d: invalid start time %q: %w", row.ID, row.StartTime, err)
	}
	end, err := parseWallClock(row.EndTime)
	if err != nil {
		return nil, fmt.Errorf("activity instance %d: invalid end time %q: %w", row.ID, row.EndTime, err)
	}
	return &slotInstance{
		ID: row.ID, Title: row.Title, Date: date, StartTime: start, EndTime: end,
		RoomID: row.RoomID, Status: row.Status, ActiveGroupID: row.ActiveGroupID, ListKind: row.ListKind,
	}, nil
}

// parseWallClock reads the owner's "HH:MM:SS" (or "HH:MM") clock value.
func parseWallClock(value string) (time.Time, error) {
	if parsed, err := time.Parse("15:04:05", value); err == nil {
		return parsed, nil
	}
	return time.Parse(timeLayout, value)
}

// loadRoster reads the roster rows of the given instances in one query.
func (s *service) loadRoster(ctx context.Context, instanceIDs []int64) ([]ports.InstanceStudent, error) {
	if len(instanceIDs) == 0 {
		return []ports.InstanceStudent{}, nil
	}
	return s.timetable.ListRoster(ctx, instanceIDs)
}

// rosterFacts projects a roster row onto the columns the care-day rule reads.
func rosterFacts(row ports.InstanceStudent) ports.RosterFacts {
	return ports.RosterFacts{
		Status: row.Status, NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
		StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
	}
}

// studentFacts projects a directory student onto the columns the enrollment
// rule reads. The directory serves enrolment dates as ISO calendar days; an
// unparsable value is a data error, never silently "unset".
func studentFacts(student *peopledirectory.Student) (ports.StudentFacts, error) {
	facts := ports.StudentFacts{Status: student.Status}
	if student.EnrolledFrom != "" {
		from, err := calendar.ParseDate(student.EnrolledFrom)
		if err != nil {
			return ports.StudentFacts{}, fmt.Errorf("student %d: invalid enrolled_from %q: %w", student.ID, student.EnrolledFrom, err)
		}
		facts.EnrolledFrom = &from
	}
	if student.EnrolledUntil != "" {
		until, err := calendar.ParseDate(student.EnrolledUntil)
		if err != nil {
			return ports.StudentFacts{}, fmt.Errorf("student %d: invalid enrolled_until %q: %w", student.ID, student.EnrolledUntil, err)
		}
		facts.EnrolledUntil = &until
	}
	return facts, nil
}

type pickupBucketConfig struct {
	ShortCutoff string
	LongCutoff  string
}

func (s *service) pickupBuckets(ctx context.Context) (pickupBucketConfig, error) {
	// s.settings is guaranteed non-nil (required by NewSlotLists). The binding
	// resolves both cutoffs from one consistent snapshot under the SHARED
	// advisory lock the settings writer takes exclusively, so an administrator
	// lowering BOTH boundaries through two individually valid writes can never
	// hand this reader an inverted pair the ordering check below would 500 on
	// (#1565 review pass 12). Shared mode means concurrent readers (/options,
	// pickup preview, exports) do NOT block one another — only a concurrent
	// cutoff writer is excluded (#1565 review).
	short, long, err := s.settings.PickupCutoffs(ctx)
	if err != nil {
		return pickupBucketConfig{}, err
	}
	shortCutoff, err := requirePickupCutoff(short, "short-day")
	if err != nil {
		return pickupBucketConfig{}, err
	}
	longCutoff, err := requirePickupCutoff(long, "long-day")
	if err != nil {
		return pickupBucketConfig{}, err
	}
	if longCutoff <= shortCutoff {
		return pickupBucketConfig{}, fmt.Errorf("long-day pickup cutoff %s must be after short-day pickup cutoff %s", longCutoff, shortCutoff)
	}
	return pickupBucketConfig{ShortCutoff: shortCutoff, LongCutoff: longCutoff}, nil
}

// requirePickupCutoff normalizes a resolved Ganztag pickup cutoff and rejects an
// empty or malformed value. The settings registry always yields a non-empty
// default for these registered keys, so an empty resolved cutoff means the
// tenant's stored configuration is corrupt — a migration or manual repair blanked
// it. Fail fast rather than silently substitute a hardcoded default and build an
// authoritative pickup cohort from values unrelated to the tenant's stored
// configuration (#1565 review pass 12).
func requirePickupCutoff(raw, label string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("%s pickup cutoff is not configured", label)
	}
	normalized, err := normalizeHHMM(raw)
	if err != nil {
		return "", fmt.Errorf("invalid %s pickup cutoff %q", label, raw)
	}
	return normalized, nil
}

type studentReadAccess struct {
	unrestricted bool
	groupIDs     map[int64]struct{}
}

func (a studentReadAccess) canReadGroup(groupID *int64) bool {
	if a.unrestricted {
		return true
	}
	if groupID == nil || a.groupIDs == nil {
		return false
	}
	_, ok := a.groupIDs[*groupID]
	return ok
}

// resolveStudentReadAccess derives which children the caller may see by name.
// Only the two legitimate restrictive conditions collapse to an empty access
// set (an account without a staff profile, a supervisor without groups);
// operational failures (repository/DB errors) propagate so the request fails
// loudly instead of returning a legitimate-looking empty list.
func (s *service) resolveStudentReadAccess(ctx context.Context) (studentReadAccess, error) {
	unrestricted, err := s.access.CanReadStudents(ctx)
	if err != nil {
		return studentReadAccess{}, err
	}
	if !unrestricted {
		return studentReadAccess{groupIDs: map[int64]struct{}{}}, nil
	}
	return studentReadAccess{unrestricted: true}, nil
}

func normalizeHHMM(value string) (string, error) {
	parsed, err := time.Parse(timeLayout, strings.TrimSpace(value))
	if err != nil {
		return "", err
	}
	return parsed.Format(timeLayout), nil
}

// listLabel derives the human list title from the already-resolved pickup
// cutoffs. It takes the snapshot rather than resolving its own so the label and
// the row collection observe the identical cutoffs within one build (#1565
// review pass 2); buckets is the zero value for slot lists, which never read it.
func (s *service) listLabel(params listRequest, buckets pickupBucketConfig) string {
	if params.Target == classday.TargetSlots {
		if params.ListKind != classday.ListKindNone {
			return params.ListKind.Label()
		}
		if len(params.InstanceIDs) == 1 {
			return "Ausgewähltes Angebot"
		}
		return "Freie Angebotsauswahl"
	}
	switch params.PickupCohort {
	case classday.PickupCohortShortDay:
		return "Ganztag bis " + buckets.ShortCutoff
	case classday.PickupCohortLongDay:
		return "Ganztag bis " + buckets.LongCutoff
	default:
		return params.Target.Label()
	}
}

func (s *service) provenance(params listRequest, listLabel string) string {
	plan := "Geplante Kinder laut Tagesplanung"
	if params.Target == classday.TargetSlots && params.ListKind != classday.ListKindNone {
		plan = "Geplante Kinder laut Tagesplanung (" + listLabel + ")"
	}
	if params.Target == classday.TargetPickupCohort {
		plan = "Geplante Kinder laut Tagesplanung und Abholzeiten (" + listLabel + ")"
	}
	switch params.Source {
	case classday.SourcePlanned:
		return plan
	case classday.SourceActual:
		return "Dokumentierte Anwesenheit am Datum"
	case classday.SourceReconciliation:
		return plan + " mit dokumentierter Anwesenheit abgeglichen"
	default:
		return ""
	}
}

// listRequest is one validated list build: the contract parameters plus the
// resolved calendar day every read is scoped to.
type listRequest struct {
	classday.Params
	date calendar.Date
}
