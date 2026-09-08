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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services/listexport"
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

// TimetableReader is the Timetable & Activities query seam the lists read.
type TimetableReader interface {
	ListActivityInstances(ctx context.Context, filter timetable.ActivityInstanceFilter) ([]timetable.ActivityInstance, error)
	ListInstanceStudents(ctx context.Context, filter timetable.InstanceStudentFilter) ([]timetable.InstanceStudent, error)
}

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
	ListExport      *listexport.RendererService
	Logger          *slog.Logger
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
	listExport      *listexport.RendererService
	logger          *slog.Logger
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
		logger:          deps.Logger,
		now:             now,
	}
}

// currentTime returns the service's notion of "now" (real clock in production,
// an injected fixed instant in tests).
func (s *service) currentTime() time.Time { return s.now() }

// todayDate returns the current calendar day in Berlin, derived from the
// service clock so the date guards move with an injected test clock.
func (s *service) todayDate() timezone.Date { return timezone.DateFromTime(s.now()) }

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
func parseDate(value classday.Date) (timezone.Date, error) {
	date, err := timezone.ParseDate(string(value))
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
	Date          timezone.Date
	StartTime     time.Time
	EndTime       time.Time
	RoomID        int64
	Status        string
	ActiveGroupID *int64
	ListKind      *string
}

// loadInstances reads every activity instance of the date in start-time
// order, exactly as every other reader of the day sees them.
func (s *service) loadInstances(ctx context.Context, date timezone.Date) ([]*slotInstance, error) {
	text := date.String()
	rows, err := s.timetable.ListActivityInstances(ctx, timetable.ActivityInstanceFilter{Date: &text, OrderByDateAndTime: true})
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

func slotInstanceFromTimetable(row timetable.ActivityInstance) (*slotInstance, error) {
	date, err := timezone.ParseDate(row.Date)
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
func (s *service) loadRoster(ctx context.Context, instanceIDs []int64) ([]timetable.InstanceStudent, error) {
	if len(instanceIDs) == 0 {
		return []timetable.InstanceStudent{}, nil
	}
	return s.timetable.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: instanceIDs, OrderByInstanceStudent: true})
}

// rosterFacts projects a roster row onto the columns the care-day rule reads.
func rosterFacts(row timetable.InstanceStudent) ports.RosterFacts {
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
		from, err := timezone.ParseDate(student.EnrolledFrom)
		if err != nil {
			return ports.StudentFacts{}, fmt.Errorf("student %d: invalid enrolled_from %q: %w", student.ID, student.EnrolledFrom, err)
		}
		facts.EnrolledFrom = &from
	}
	if student.EnrolledUntil != "" {
		until, err := timezone.ParseDate(student.EnrolledUntil)
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
	date timezone.Date
}

func (s *service) BuildList(ctx context.Context, params classday.Params) (*classday.Result, error) {
	if err := s.requireTimetableEnabled(ctx); err != nil {
		return nil, err
	}
	if !s.configured() {
		return nil, fmt.Errorf("slot list service is not configured")
	}
	date, err := parseDate(params.Date)
	if err != nil {
		return nil, err
	}
	return s.buildList(ctx, listRequest{Params: params, date: date})
}

func (s *service) buildList(ctx context.Context, params listRequest) (*classday.Result, error) {
	if !params.Target.Valid() {
		return nil, fmt.Errorf("unknown list target %q", params.Target)
	}
	if !params.Source.Valid() {
		return nil, fmt.Errorf("unknown source %q", params.Source)
	}
	if params.Target == classday.TargetPickupCohort && !params.PickupCohort.Valid() {
		return nil, fmt.Errorf("unknown pickup cohort %q", params.PickupCohort)
	}
	if !params.ListKind.Valid() {
		return nil, fmt.Errorf("unknown list kind %q", params.ListKind)
	}
	if params.Target != classday.TargetSlots && params.ListKind != classday.ListKindNone {
		return nil, fmt.Errorf("list kind %q is not valid for target %q", params.ListKind, params.Target)
	}
	if !params.GroupBy.ValidFor(params.Target) {
		return nil, fmt.Errorf("grouping %q is not valid for target %q", params.GroupBy, params.Target)
	}
	// A Ganztag pickup plan cannot be reconstructed for a past date (the pickup
	// schedule has no history) — refuse rather than project the current schedule
	// backwards. Slot-based lists stay available for any date (#1565 review).
	if params.Target == classday.TargetPickupCohort && params.date.Before(s.todayDate()) {
		return nil, classday.ErrPickupCohortPastDate
	}
	// A reconciliation on a strictly-future date has no presence evidence to merge
	// against, so every planned child would be labelled "Fehlt". Refuse it rather
	// than emit a list that reports the whole group missing before the day starts
	// (#1565 review). Today is fine — evidence accrues through the day; Plan/Ist
	// stay available for any date. Slots that have not started yet *on today* are
	// handled per-slot in collectSlotEntries (a not-yet-begun occurrence is
	// excluded from the merge, not reported as a group of missing children).
	if params.Source == classday.SourceReconciliation && params.date.After(s.todayDate()) {
		return nil, classday.ErrReconciliationFutureDate
	}
	access, err := s.resolveStudentReadAccess(ctx)
	if err != nil {
		return nil, err
	}

	// Resolve the pickup-cohort cutoffs ONCE per build and reuse the snapshot for
	// both the list label/provenance and the row collection. Resolving them twice
	// (label, then collectPickupEntries) would let an administrator's mid-build
	// cutoff change land between the two reads under READ COMMITTED, so the header
	// could name a different threshold than the rows were bucketed by (#1565 review
	// pass 2). Only pickup lists need the cutoffs; a slot list neither reads them
	// nor should fail on an invalid pickup config.
	var pickupCfg pickupBucketConfig
	if params.Target == classday.TargetPickupCohort {
		pickupCfg, err = s.pickupBuckets(ctx)
		if err != nil {
			return nil, err
		}
	}

	listLabel := s.listLabel(params, pickupCfg)

	result := &classday.Result{
		Date:         params.Date.String(),
		Target:       params.Target,
		PickupCohort: params.PickupCohort,
		ListKind:     params.ListKind,
		ListLabel:    listLabel,
		Source:       params.Source,
		GroupBy:      params.GroupBy,
		Provenance:   s.provenance(params, listLabel),
		Slots:        []classday.Slot{},
		Rows:         []classday.Row{},
	}

	var entries []mergedEntry
	if params.Target == classday.TargetPickupCohort {
		entries, err = s.collectPickupEntries(ctx, params, pickupCfg, result)
	} else {
		entries, err = s.collectSlotEntries(ctx, params, result)
	}
	if err != nil {
		return nil, err
	}

	// Full (unfiltered) rows for the requested source — used both to derive the
	// available group/class options and as the input to the row filters.
	allRows, err := s.enrichEntries(ctx, entries, params.Source, access, params.date)
	if err != nil {
		return nil, err
	}
	result.Groups, result.Classes = availableOptions(allRows)

	rows := filterRows(allRows, params)
	applyGrouping(rows, params.GroupBy)
	sort.SliceStable(rows, func(i, j int) bool {
		// Keep grouped rows contiguous: section heading first, then the same
		// slot/name order within each section.
		if rows[i].GroupTitle != rows[j].GroupTitle {
			return rows[i].GroupTitle < rows[j].GroupTitle
		}
		if rows[i].Slot != rows[j].Slot {
			return rows[i].Slot < rows[j].Slot
		}
		if rows[i].Name != rows[j].Name {
			return rows[i].Name < rows[j].Name
		}
		// Stable ID tie-breakers: two rows can share group title, slot and full
		// name — e.g. two identically named unplanned attendees in one slot. Their
		// input order comes from map iteration / an incompletely ordered query,
		// while listSignature hashes InstanceID and StudentID positionally, so
		// without a deterministic order unchanged data could produce different
		// preview/export signatures and trigger false drift refusals (#1565 review
		// pass 9).
		if rows[i].InstanceID != rows[j].InstanceID {
			return rows[i].InstanceID < rows[j].InstanceID
		}
		return rows[i].StudentID < rows[j].StudentID
	})
	result.Rows = rows
	result.Counters = countRows(rows, params.Source)
	// Fingerprint the exact rendered header (title + every export filter line)
	// so the signature reflects the "Enthalten" slot summary and the other
	// metadata lines the export prints, not just rows and counters (#1565 review
	// pass 7). Computed here because the header needs Params.
	result.ExportHeaderSignature = strings.Join(
		append([]string{documentTitle(result)}, exportFilters(params, result)...),
		"\x1f",
	)
	result.Signature = listSignature(result)
	return result, nil
}

// listSignature is a stable content hash of the rendered list: its label,
// provenance, the precomputed export header (title + filter lines, which carry
// the "Enthalten" slot summary — see Result.exportHeaderSig), counters and every
// row's identity, placement and status. Rows are already sorted deterministically
// by BuildList, so a positional walk is stable across two builds of the same
// unchanged data. Unit/record separators (0x1f / 0x1e) delimit fields so no value
// can be confused with a boundary. See the Result.Signature doc for how the
// export drift guard uses it (#1565 review pass 2 + pass 7).
func listSignature(r *classday.Result) string {
	var b strings.Builder
	b.WriteString(r.ListLabel)
	b.WriteByte('\x1e')
	b.WriteString(r.Provenance)
	b.WriteByte('\x1e')
	// The rendered document header (title + export filter lines), precomputed in
	// BuildList. Binds the signature to the "Enthalten" slot summary and the
	// date/group/class/grouping metadata the export prints — none of which the
	// row/counter hash below covers (#1565 review pass 7).
	b.WriteString(r.ExportHeaderSignature)
	b.WriteByte('\x1e')
	fmt.Fprintf(&b, "%d\x1f%d\x1f%d\x1f%d\x1f%d\x1e",
		r.Counters.Planned, r.Counters.Present, r.Counters.Missing,
		r.Counters.Excused, r.Counters.Unplanned)
	for _, row := range r.Rows {
		fmt.Fprintf(&b,
			"%d\x1f%s\x1f%s\x1f%s\x1f%s\x1f%d\x1f%s\x1f%s\x1f%s\x1f%s\x1f%t\x1f%t\x1f%t\x1f%t\x1f%s\x1e",
			row.StudentID, row.Name, row.SchoolClass, row.GroupName,
			groupIDSignatureField(row.GroupID), row.InstanceID, row.Slot,
			row.RoomName, row.PickupTime, row.StatusLabel, row.Planned,
			row.Present, row.Unplanned, row.Excused, row.GroupTitle)
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func groupIDSignatureField(id *int64) string {
	if id == nil {
		return ""
	}
	return strconv.FormatInt(*id, 10)
}

func (s *service) ListOptions(ctx context.Context, requested classday.Date) (*classday.OptionsResult, error) {
	if err := s.requireTimetableEnabled(ctx); err != nil {
		return nil, err
	}
	if !s.configured() {
		return nil, fmt.Errorf("slot list service is not configured")
	}
	date, err := parseDate(requested)
	if err != nil {
		return nil, err
	}
	// One pass over the day's instances instead of re-running the full list
	// builder once per list kind: on a normal school day the old shape issued
	// hundreds of per-slot roster/visit queries inside the tenant transaction
	// before the frontend's preview request repeated the same work.
	instances, err := s.loadInstances(ctx, date)
	if err != nil {
		return nil, err
	}
	slots := make([]classday.Slot, 0, len(instances))
	kindInstanceIDs := map[classday.ListKind][]int64{}
	// Resolve each slot's room name and list kind into the options payload so the
	// frontend can detect a room reassignment/rename or a list_kind change that
	// leaves the active instance-ID set unchanged before it exports (#1565 review
	// pass 4). The room lookup is cached per build, matching collectSlotEntries.
	roomCache := map[int64]string{}
	for _, inst := range instances {
		// Cancelled occurrences are not list candidates. Filter them before room
		// resolution and before publishing Slots so omitted instance_ids means
		// "all non-cancelled slots" and a stale/direct cancelled selection cannot
		// be offered back to the client.
		if inst.Status == timetable.InstanceStatusCancelled {
			continue
		}
		roomName, err := s.lookupRoomName(ctx, inst.RoomID, roomCache)
		if err != nil {
			return nil, err
		}
		listKind := ""
		if inst.ListKind != nil {
			listKind = *inst.ListKind
		}
		slots = append(slots, classday.Slot{
			InstanceID: inst.ID,
			Title:      inst.Title,
			TimeRange:  fmt.Sprintf("%s\u2013%s", inst.StartTime.Format(timeLayout), inst.EndTime.Format(timeLayout)),
			Status:     inst.Status,
			ListKind:   listKind,
			RoomName:   roomName,
		})
		if inst.ListKind == nil {
			continue
		}
		kind := classday.ListKind(*inst.ListKind)
		if kind != classday.ListKindNone && kind.Valid() {
			kindInstanceIDs[kind] = append(kindInstanceIDs[kind], inst.ID)
		}
	}

	// Options counts must respect the same GDPR read scope and date-effective
	// enrollment as /preview, or the availability hints would leak counts
	// derived from unsupervised children or from children not enrolled on the
	// requested date (#1565 review). Build the readable + enrolled student set
	// once and gate every aggregation on it.
	access, err := s.resolveStudentReadAccess(ctx)
	if err != nil {
		return nil, err
	}
	students, err := s.listEligibleStudents(ctx, date, nil)
	if err != nil {
		return nil, err
	}
	studentIDs := make([]int64, 0, len(students))
	readable := make(map[int64]struct{}, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
		if access.canReadGroup(student.GroupID) {
			readable[student.ID] = struct{}{}
		}
	}

	// The date's care-day verdict, resolved once for every eligible child and
	// reused by the roster counts and the pickup cohorts below. An assignment
	// alone does not book a child into care on a given weekday (#1747): the
	// not_scheduled marker is only frozen onto the row when the block ends, so a
	// whole-group/year assignment would otherwise inflate the planned counts on
	// the days its members are not scheduled for.
	careDays := map[int64]ports.CareDay{}
	if len(studentIDs) > 0 {
		careDays, err = s.careDays.ResolveForDate(ctx, studentIDs, date)
		if err != nil {
			return nil, fmt.Errorf("resolve care days: %w", err)
		}
	}
	completedByInstance := make(map[int64]bool, len(instances))
	for _, inst := range instances {
		completedByInstance[inst.ID] = inst.Status == timetable.InstanceStatusCompleted
	}

	// One bulk roster load over every classified instance for the per-kind
	// planned row counts. Mirrors collectSlotEntries' planned semantics:
	// walk-in rows (is_unplanned, #1913) and non-booking rows — the frozen
	// not_scheduled marker or the live care-day verdict (#1747) — are not
	// planned children; only readable children count.
	classifiedIDs := make([]int64, 0)
	for _, ids := range kindInstanceIDs {
		classifiedIDs = append(classifiedIDs, ids...)
	}
	// Visit-derived presence for the classified instances, mirroring the list
	// builder's loadSlotPresence. The builder observes attendance through
	// active.visits (presentByGroup), so classifyPlannedRow can emit a child who
	// checked in on a CANCELLED care day as unplanned presence even when the
	// deliberately best-effort attendance sync left the roster row Expected instead
	// of flipping it to Present (visit created, sync failed — #1439/checkin). The
	// roster-only Status==Present check below cannot see that visit-only presence,
	// so it would miscount the child as a planned "Abgemeldet" row and overreport
	// /options.list_kinds[].row_count versus what Plan preview/export render. Load
	// the same visit evidence here and fold it into the presence signal (#1565
	// review pass 2 P2).
	activeGroupByInstance := make(map[int64]*int64, len(instances))
	for _, inst := range instances {
		activeGroupByInstance[inst.ID] = inst.ActiveGroupID
	}
	presentByGroup := map[int64]map[int64]bool{}
	presentViaVisit := func(instanceID, studentID int64) bool {
		activeGroupID := activeGroupByInstance[instanceID]
		if activeGroupID == nil {
			return false
		}
		byStudent := presentByGroup[*activeGroupID]
		return byStudent != nil && byStudent[studentID]
	}
	plannedByInstance := map[int64]int{}
	if len(classifiedIDs) > 0 {
		rosterRows, err := s.loadRoster(ctx, classifiedIDs)
		if err != nil {
			return nil, err
		}
		// Bulk-load the visits of every classified instance that has been started,
		// exactly as loadSlotPresence does: presence is keyed by active-group ID
		// (only a started occurrence carries one), and nil presence simply
		// contributes no evidence.
		if s.presence != nil {
			activeGroupIDs := make([]int64, 0, len(classifiedIDs))
			for _, id := range classifiedIDs {
				if activeGroupID := activeGroupByInstance[id]; activeGroupID != nil {
					activeGroupIDs = append(activeGroupIDs, *activeGroupID)
				}
			}
			if len(activeGroupIDs) > 0 {
				visits, err := s.presence.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: activeGroupIDs})
				if err != nil {
					return nil, err
				}
				for _, visit := range visits {
					byStudent := presentByGroup[visit.ActiveGroupID]
					if byStudent == nil {
						byStudent = map[int64]bool{}
						presentByGroup[visit.ActiveGroupID] = byStudent
					}
					byStudent[visit.StudentID] = true
				}
			}
		}
		for _, row := range rosterRows {
			if row.IsUnplanned || row.NotScheduled {
				continue
			}
			if _, ok := readable[row.StudentID]; !ok {
				continue
			}
			completed := completedByInstance[row.InstanceID]
			// Mirror collectSlotEntries exactly, including its RAW care-day
			// handling on unfinished occurrences: a bulk-assigned child who is not
			// booked for the weekday but checks in flips the row to present, so
			// AttendanceRowCareDay reports unknown (a real status tells its own
			// story). collectSlotEntries drops that row as present-only/unplanned
			// via the raw `CareDayNotScheduled` branch, so keying the count on the
			// status-gated verdict alone would claim a planned child the Plan
			// preview and export omit (#1565 review). A manual override still wins,
			// and a completed occurrence trusts its frozen marker (folded into
			// AttendanceRowCareDay below), never the live plan.
			if !completed && row.ManualStatusAt == nil &&
				careDays[row.StudentID] == ports.CareDayNotScheduled {
				continue
			}
			// A cancelled care day the child ATTENDED anyway is unplanned presence,
			// not a planned row: classifyPlannedRow's cancelled branch emits such a
			// present child present-only (exactly like the IsUnplanned and
			// not_scheduled branches), never the planned "Abgemeldet" shape, so the
			// generic increment below would miscount it as planned and overreport
			// /options versus the Plan preview and export. Key on the RAW cancellation
			// verdict (a manual override still wins and is excluded), mirroring
			// classifyPlannedRow. Presence is derived exactly as classifyPlannedRows
			// does — `presentSet[student] || row.Status == Present` — so a normal
			// check-in that flipped the row to Present AND a check-in whose row status
			// never updated (the best-effort attendance sync failed but the visit
			// exists) are both recognized as unplanned presence. Without the visit
			// leg, a visit-only presence would count as planned here while the builder
			// classifies it unplanned, so the signatures diverge (#1565 review pass 2
			// P2). A cancelled NO-SHOW keeps its Expected/absent status, has no visit,
			// and still counts below as the "Abgemeldet" planned row it prints as
			// (#1565 review pass 1).
			if row.ManualStatusAt == nil &&
				careDays[row.StudentID] == ports.CareDayCancelled &&
				(row.Status == timetable.InstanceAttendancePresent ||
					presentViaVisit(row.InstanceID, row.StudentID)) {
				continue
			}
			verdict := s.rules.RowCareDay(completed, rosterFacts(row), careDays[row.StudentID])
			// A genuine non-booking (not_scheduled) — whether the row still reads
			// Expected or was already stamped absent by a broad status day on a day
			// the child was never scheduled — is not a planned row and must not be
			// counted. A signed-off cancellation IS retained in collectSlotEntries
			// as a planned "Abgemeldet" row, so it must count here too, otherwise
			// /options underreports the classified list versus preview/export.
			if verdict == ports.CareDayNotScheduled {
				continue
			}
			plannedByInstance[row.InstanceID]++
		}
	}
	listKinds := make([]classday.ListKindOption, 0, len(classday.AllListKinds))
	for _, kind := range classday.AllListKinds {
		ids := kindInstanceIDs[kind]
		rowCount := 0
		for _, id := range ids {
			rowCount += plannedByInstance[id]
		}
		listKinds = append(listKinds, classday.ListKindOption{
			Kind:      kind,
			Label:     kind.Label(),
			Available: len(ids) > 0,
			SlotCount: len(ids),
			RowCount:  rowCount,
		})
	}

	// One pickup sweep for both Ganztag cohorts over the same readable +
	// enrolled set, so the availability hint matches what /preview would show.
	// A past date is skipped entirely: BuildList refuses past pickup lists (the
	// schedule has no history), so their availability is always zero (#1565
	// review).
	buckets, err := s.pickupBuckets(ctx)
	if err != nil {
		return nil, err
	}
	cohortCounts := map[classday.PickupCohort]int{}
	if !date.Before(s.todayDate()) && len(studentIDs) > 0 {
		pickupTimes, err := s.effectiveTimes.PickupTimes(ctx, studentIDs, date)
		if err != nil {
			return nil, err
		}
		// Same cancelled-care fallback as collectPickupEntries: a cleared
		// same-day exception drops the effective time, but a cancelled verdict
		// (resolved once above into careDays) keeps the child visible via the
		// regular weekly bucket. Without this, the card reported zero while the
		// preview showed the signed-off child.
		regularBucket, err := s.regularPickupBucket(ctx, studentIDs, date)
		if err != nil {
			return nil, err
		}
		for _, student := range students {
			if _, ok := readable[student.ID]; !ok {
				continue
			}
			if careDays[student.ID] == ports.CareDayNotScheduled {
				continue
			}
			cancelled := careDays[student.ID] == ports.CareDayCancelled
			hhmm := cohortPickupTime(cancelled, pickupTimes[student.ID], regularBucket[student.ID])
			if hhmm == "" {
				continue
			}
			for _, cohort := range []classday.PickupCohort{classday.PickupCohortShortDay, classday.PickupCohortLongDay} {
				if pickupMatchesCohort(cohort, hhmm, buckets) {
					cohortCounts[cohort]++
				}
			}
		}
	}
	cohortLabels := map[classday.PickupCohort]string{
		classday.PickupCohortShortDay: "Ganztag bis " + buckets.ShortCutoff,
		classday.PickupCohortLongDay:  "Ganztag bis " + buckets.LongCutoff,
	}
	cohorts := make([]classday.PickupCohortOption, 0, 2)
	for _, cohort := range []classday.PickupCohort{classday.PickupCohortShortDay, classday.PickupCohortLongDay} {
		cohorts = append(cohorts, classday.PickupCohortOption{
			Cohort:    cohort,
			Label:     cohortLabels[cohort],
			Available: cohortCounts[cohort] > 0,
			RowCount:  cohortCounts[cohort],
		})
	}
	return &classday.OptionsResult{Date: date.String(), Slots: slots, PickupCohorts: cohorts, ListKinds: listKinds}, nil
}

// applyGrouping stamps each row's GroupTitle from the chosen dimension. With
// GroupByNone every title stays empty (a flat list). Rows whose group value is
// empty fall into a generic "Ohne …" bucket so they never silently vanish.
func applyGrouping(rows []classday.Row, groupBy classday.GroupBy) {
	if groupBy == classday.GroupByNone {
		return
	}
	// GroupBySlot keys on the Slot label, but that label is NOT unique across
	// activity instances: two concrete instances can share a title and time range
	// (parallel offerings in different rooms, or two templates). Since both the
	// export's repeated-marker suppression and the frontend's group_title
	// bucketing key sections off the GroupTitle string, an undisambiguated label
	// would fold distinct instances into one section. Precompute a per-instance
	// heading suffix that only fires when a label actually collides (#1565 review
	// pass 2).
	slotSuffix := slotHeadingDisambiguation(rows, groupBy)
	for i := range rows {
		var value string
		switch groupBy {
		case classday.GroupBySlot:
			value = rows[i].Slot
			value += slotSuffix[rows[i].InstanceID]
		case classday.GroupByRoom:
			value = rows[i].RoomName
		case classday.GroupByClass:
			value = rows[i].SchoolClass
		case classday.GroupByPickupTime:
			value = rows[i].PickupTime
		}
		if value == "" {
			value = "Ohne " + groupBy.Label()
		}
		rows[i].GroupTitle = groupBy.Label() + ": " + value
	}
}

// slotHeadingDisambiguation returns a per-instance heading suffix for GroupBySlot
// so distinct activity instances that share one Slot label never merge into a
// single section. Instances whose label is already unique get no suffix (the
// map returns "" for them), so the common case renders exactly as before. When a
// label is shared by several instances, each colliding instance is tagged with
// its room name where that uniquely identifies it, otherwise a running ordinal —
// guaranteeing a distinct, deterministic heading without leaking raw IDs. Returns
// nil for any non-slot grouping (no suffix ever applied).
func slotHeadingDisambiguation(rows []classday.Row, groupBy classday.GroupBy) map[int64]string {
	if groupBy != classday.GroupBySlot {
		return nil
	}
	// Per label: the distinct instance IDs (ascending, for stable ordinals) and
	// each instance's room.
	type labelInfo struct {
		ids   []int64
		room  map[int64]string
		known map[int64]struct{}
	}
	byLabel := map[string]*labelInfo{}
	for _, r := range rows {
		li := byLabel[r.Slot]
		if li == nil {
			li = &labelInfo{room: map[int64]string{}, known: map[int64]struct{}{}}
			byLabel[r.Slot] = li
		}
		if _, ok := li.known[r.InstanceID]; ok {
			continue
		}
		li.known[r.InstanceID] = struct{}{}
		li.ids = append(li.ids, r.InstanceID)
		li.room[r.InstanceID] = r.RoomName
	}
	suffix := map[int64]string{}
	for _, li := range byLabel {
		if len(li.ids) < 2 {
			continue // unique label — no disambiguation, heading stays clean
		}
		sort.Slice(li.ids, func(i, j int) bool { return li.ids[i] < li.ids[j] })
		roomCount := map[string]int{}
		for _, id := range li.ids {
			roomCount[li.room[id]]++
		}
		// Two passes with a shared `used` set so the room and ordinal suffixes can
		// never render the same string. A numeric room name like "1" produces the
		// suffix " (1)", which is textually identical to the first ordinal " (1)";
		// emitting both would hand two distinct instances the same GroupTitle and
		// re-merge the sections this function exists to split. First assign every
		// uniquely-identifying room its " (room)" suffix and record it; then fill the
		// remaining instances with running ordinals, SKIPPING any ordinal string a
		// room suffix already claimed. This keeps both forms clean (no verbose prefix,
		// no leaked IDs) while guaranteeing distinct, deterministic headings
		// (#1565 review pass 2 follow-up).
		used := map[string]struct{}{}
		for _, id := range li.ids {
			if room := li.room[id]; room != "" && roomCount[room] == 1 {
				s := " (" + room + ")"
				suffix[id] = s
				used[s] = struct{}{}
			}
		}
		ordinal := 0
		for _, id := range li.ids {
			if _, done := suffix[id]; done {
				continue
			}
			var s string
			for {
				ordinal++
				s = fmt.Sprintf(" (%d)", ordinal)
				if _, taken := used[s]; !taken {
					break
				}
			}
			suffix[id] = s
			used[s] = struct{}{}
		}
	}
	return suffix
}

// availableOptions derives the distinct education groups and school classes
// present in the unfiltered rows, sorted for stable UI dropdowns.
func availableOptions(rows []classday.Row) ([]classday.GroupOption, []string) {
	groupByID := map[int64]string{}
	classSet := map[string]struct{}{}
	for _, row := range rows {
		if row.GroupID != nil && row.GroupName != "" {
			groupByID[*row.GroupID] = row.GroupName
		}
		if row.SchoolClass != "" {
			classSet[row.SchoolClass] = struct{}{}
		}
	}
	groups := make([]classday.GroupOption, 0, len(groupByID))
	for id, name := range groupByID {
		groups = append(groups, classday.GroupOption{ID: id, Name: name})
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
	classes := make([]string, 0, len(classSet))
	for class := range classSet {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	return groups, classes
}

// filterRows applies the offering/group/class filters (AND). An empty filter
// slice means "no restriction" on that dimension.
func filterRows(rows []classday.Row, params listRequest) []classday.Row {
	if params.Target == classday.TargetSlots && params.InstanceIDsSet && len(params.InstanceIDs) == 0 {
		return []classday.Row{}
	}
	instanceSet := int64Set(params.InstanceIDs)
	groupSet := int64Set(params.GroupIDs)
	classSet := make(map[string]struct{}, len(params.Classes))
	for _, c := range params.Classes {
		classSet[c] = struct{}{}
	}

	out := make([]classday.Row, 0, len(rows))
	for _, row := range rows {
		// InstanceIDs are a slot-list filter only; they are ignored for pickup
		// cohorts (whose rows carry InstanceID 0), where a non-empty selection
		// would otherwise drop every row (#1565 review).
		if params.Target == classday.TargetSlots && len(instanceSet) > 0 && !instanceSet[row.InstanceID] {
			continue
		}
		if len(groupSet) > 0 && (row.GroupID == nil || !groupSet[*row.GroupID]) {
			continue
		}
		if len(classSet) > 0 {
			if _, ok := classSet[row.SchoolClass]; !ok {
				continue
			}
		}
		out = append(out, row)
	}
	return out
}

func int64StructSet(ids []int64) map[int64]struct{} {
	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func int64Set(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// mergedEntry is one (slot, student) pair before person/group enrichment.
type mergedEntry struct {
	StudentID        int64
	InstanceID       int64 // 0 for pickup-based cohorts
	SlotLabel        string
	RoomName         string // planned room of the slot; "" for pickup cohorts
	PickupTime       string
	Planned          bool
	Present          bool
	PlannedStatus    string  // schedule.instance_students.status, "" for pickup cohorts
	PlannedSubstatus *string // schedule.instance_students.substatus, nil for pickup cohorts
}

// collectSlotEntries derives the cohort from materialized activity instances
// (Plan) and their bridged active-group visits (Ist). Every visit reachable
// through the occurrence's 1:1 active-group bridge counts as present, including
// historical checked-out visits and sessions run outside the planned window, so
// past lists and late/early starts keep their attendance. Cancelled occurrences
// are excluded before this merge, regardless of whether they still carry an
// active-group bridge or historical visits.
func (s *service) collectSlotEntries(ctx context.Context, params listRequest, result *classday.Result) ([]mergedEntry, error) {
	contexts, deferredInstances, err := s.collectSlotContexts(ctx, params, result)
	if err != nil {
		return nil, err
	}
	rosterByInstance, careDay, presentByGroup, err := s.loadSlotPresence(ctx, contexts, params.date)
	if err != nil {
		return nil, err
	}
	entries := []mergedEntry{}
	for _, p := range contexts {
		entries = append(entries, s.mergeSlotInstance(p, rosterByInstance, careDay, presentByGroup, deferredInstances, result)...)
	}
	return entries, nil
}

// slotContext is one activity instance selected for row collection, with its
// pre-resolved room name and printable slot label.
type slotContext struct {
	inst      *slotInstance
	slotLabel string
	roomName  string
}

// collectSlotContexts runs the first pass over the date's instances: it publishes
// every matching non-cancelled slot as selectable context on result.Slots,
// returns the selected instances whose rosters the merge will actually read,
// and the set of deferred (not-yet-started reconciliation) occurrences.
func (s *service) collectSlotContexts(ctx context.Context, params listRequest, result *classday.Result) ([]slotContext, map[int64]struct{}, error) {
	instances, err := s.loadInstances(ctx, params.date)
	if err != nil {
		return nil, nil, err
	}

	// A non-nil set restricts data collection to the selected instances;
	// nil means "all" (the default when no slots are explicitly selected).
	// The option list (result.Slots) is always populated for every matching
	// instance regardless — only the expensive per-instance roster/visit
	// reads are gated, so selecting one slot no longer costs the same as
	// selecting all, and an explicit empty selection issues no per-slot reads.
	var selected map[int64]struct{}
	if params.InstanceIDsSet || len(params.InstanceIDs) > 0 {
		selected = int64StructSet(params.InstanceIDs)
	}

	roomCache := map[int64]string{}
	process := make([]slotContext, 0, len(instances))
	// deferredInstances flags reconciliation occurrences reached before their
	// scheduled start with no active group yet. They still enter `process` (their
	// roster may hold manually-recorded presence), but the merge suppresses their
	// void plan so no not-yet-arrived child prints as "Fehlt" (#1565 review pass 2).
	var deferredInstances map[int64]struct{}
	for _, inst := range instances {
		// Enforce the non-cancelled contract before list-kind matching, option
		// publication and selection. This also neutralizes a caller passing a
		// cancelled instance ID directly.
		if inst.Status == timetable.InstanceStatusCancelled {
			continue
		}
		if !instanceMatchesListKind(inst, params.ListKind) {
			continue
		}
		sc, deferred, err := s.buildSlotContext(ctx, inst, params, roomCache, result)
		if err != nil {
			return nil, nil, err
		}
		// deferredInstances is recorded before the selection gate — a non-selected
		// deferred slot is never looked up, but keeping the order identical to the
		// pre-split code avoids any behavioral surprise.
		if deferred {
			if deferredInstances == nil {
				deferredInstances = map[int64]struct{}{}
			}
			deferredInstances[inst.ID] = struct{}{}
		}
		if selected != nil {
			if _, ok := selected[inst.ID]; !ok {
				continue
			}
		}
		process = append(process, sc)
	}
	return process, deferredInstances, nil
}

// buildSlotContext publishes one matching instance as selectable context on
// result.Slots and returns its slotContext plus whether it is a deferred
// (not-yet-started reconciliation) occurrence.
func (s *service) buildSlotContext(ctx context.Context, inst *slotInstance, params listRequest, roomCache map[int64]string, result *classday.Result) (slotContext, bool, error) {
	roomName, err := s.lookupRoomName(ctx, inst.RoomID, roomCache)
	if err != nil {
		return slotContext{}, false, err
	}
	listKind := ""
	if inst.ListKind != nil {
		listKind = *inst.ListKind
	}
	result.Slots = append(result.Slots, classday.Slot{
		InstanceID: inst.ID,
		Title:      inst.Title,
		TimeRange:  fmt.Sprintf("%s–%s", inst.StartTime.Format(timeLayout), inst.EndTime.Format(timeLayout)),
		Status:     inst.Status,
		ListKind:   listKind,
		RoomName:   roomName,
	})
	// A reconciliation reaching an occurrence before its scheduled start, with
	// no active group yet, has no live presence to merge against, so every
	// planned child would collapse to Present=false and render as "Fehlt" — a
	// safety-relevant false missing-child list before the activity begins. Flag
	// it deferred so the merge suppresses those planned no-shows (while still
	// surfacing any manually-present child) and the export summary does not
	// count a slot that produced no rows. Gate strictly on occurrences that have
	// NOT actually started: a slot Start()ed manually before its nominal time
	// already carries an ActiveGroupID and real visits, so it stays on the
	// normal path. This only ever fires for reconciliation on today — a future
	// date is refused outright in BuildList; Plan/Ist are unaffected.
	deferred := params.Source == classday.SourceReconciliation && inst.ActiveGroupID == nil
	if deferred {
		if start, _ := instanceTimeRange(inst); !s.currentTime().Before(start) {
			deferred = false
		}
	}
	slotLabel := fmt.Sprintf("%s (%s–%s)", inst.Title, inst.StartTime.Format(timeLayout), inst.EndTime.Format(timeLayout))
	return slotContext{inst: inst, slotLabel: slotLabel, roomName: roomName}, deferred, nil
}

// loadSlotPresence bulk-loads the rosters, the shared care-day verdict, and the
// visit-derived presence for every instance the merge will process.
func (s *service) loadSlotPresence(ctx context.Context, process []slotContext, date timezone.Date) (
	map[int64][]timetable.InstanceStudent,
	map[int64]ports.CareDay,
	map[int64]map[int64]bool,
	error,
) {
	// Bulk-load the rosters of every instance we will process, then resolve the
	// date's care-day verdict once for all planned children. An assignment alone
	// does not book a child into care on a given weekday (#1747): a whole-group
	// or whole-year assignment lists members on the days they are not scheduled
	// for, and the not_scheduled marker is only frozen onto the row when the
	// block ends. Without the live verdict such a child reaches the default
	// branch below, counts as planned, and shows as "Fehlt" in the Abgleich
	// though they were never booked.
	rosterByInstance := make(map[int64][]timetable.InstanceStudent, len(process))
	careDay := map[int64]ports.CareDay{}
	// presentByGroup maps an active-group ID to the students seen present in it.
	// A slot only has visit evidence once it carries an ActiveGroupID, so this is
	// keyed by group, not instance; instances without one (never started — e.g.
	// most Plan requests) contribute nothing and cost no query.
	presentByGroup := map[int64]map[int64]bool{}
	if len(process) == 0 {
		return rosterByInstance, careDay, presentByGroup, nil
	}
	instanceIDs := make([]int64, 0, len(process))
	activeGroupIDs := make([]int64, 0, len(process))
	for _, p := range process {
		instanceIDs = append(instanceIDs, p.inst.ID)
		if p.inst.ActiveGroupID != nil {
			activeGroupIDs = append(activeGroupIDs, *p.inst.ActiveGroupID)
		}
	}
	rosterRows, err := s.loadRoster(ctx, instanceIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	studentIDSet := map[int64]struct{}{}
	for _, row := range rosterRows {
		rosterByInstance[row.InstanceID] = append(rosterByInstance[row.InstanceID], row)
		studentIDSet[row.StudentID] = struct{}{}
	}
	studentIDs := make([]int64, 0, len(studentIDSet))
	for id := range studentIDSet {
		studentIDs = append(studentIDs, id)
	}
	careDay, err = s.careDays.ResolveForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("resolve care days: %w", err)
	}
	// Bulk-load every started slot's visits in one query rather than probing
	// each instance individually inside the merge loop below — a historical
	// day with many slots otherwise runs N sequential visit queries per
	// preview/export/filter change.
	//
	// The instance↔active.group bridge is 1:1 (a UNIQUE partial index on
	// schedule.activity_instances.active_group_id) and the active.group is
	// created fresh when the occurrence is started, so every visit reachable
	// through it is that occurrence's own attendance — it is deliberately NOT
	// time-filtered against the planned window. Staff may Start an instance
	// late, or start and complete it early: InstanceService.Start permits the
	// transition regardless of the nominal StartTime/EndTime, which pushes
	// those visits wholly outside the scheduled window. Comparing them to the
	// planned times would drop documented attendance, omitting present
	// children from Ist lists and printing attended children as "Fehlt" in the
	// Abgleich (#1565 review pass 2).
	visits, err := s.presence.ListVisits(ctx, studentpresence.VisitFilter{ActiveGroupIDs: activeGroupIDs})
	if err != nil {
		return nil, nil, nil, err
	}
	for _, visit := range visits {
		byStudent := presentByGroup[visit.ActiveGroupID]
		if byStudent == nil {
			byStudent = map[int64]bool{}
			presentByGroup[visit.ActiveGroupID] = byStudent
		}
		byStudent[visit.StudentID] = true
	}
	return rosterByInstance, careDay, presentByGroup, nil
}

// mergeSlotInstance merges one instance's roster (Plan) with its visit-derived
// presence (Ist) into display entries and records the export "Enthalten"
// accounting for void-plan slots.
func (s *service) mergeSlotInstance(
	p slotContext,
	rosterByInstance map[int64][]timetable.InstanceStudent,
	careDay map[int64]ports.CareDay,
	presentByGroup map[int64]map[int64]bool,
	deferredInstances map[int64]struct{},
	result *classday.Result,
) []mergedEntry {
	inst := p.inst
	planned := rosterByInstance[inst.ID]
	_, deferred := deferredInstances[inst.ID]
	// A reconciliation that reaches an occurrence before its scheduled start
	// has no genuine expectation yet, so its roster may not print as "Fehlt".
	// Cancelled occurrences never reach this method.
	// Presence was bulk-loaded above; a slot with no active group (never
	// started) simply has no entry and yields a nil map (safe to index).
	var presentSet map[int64]bool
	if inst.ActiveGroupID != nil {
		presentSet = presentByGroup[*inst.ActiveGroupID]
	}
	completed := inst.Status == timetable.InstanceStatusCompleted

	entries := []mergedEntry{}
	seenPlanned := make(map[int64]struct{}, len(planned))
	retainedRows := false
	if deferred {
		retained, any := s.retainDeferredRows(p, planned, careDay, seenPlanned)
		entries = append(entries, retained...)
		retainedRows = any
		planned = nil
	}
	entries = append(entries, s.classifyPlannedRows(p, planned, presentSet, completed, careDay, seenPlanned)...)
	unseen, anyPresent := appendUnseenPresent(p, presentSet, seenPlanned)
	entries = append(entries, unseen...)
	if anyPresent {
		retainedRows = true
	}
	recordDeferredAccounting(result, inst.ID, deferred, retainedRows)
	return entries
}

// classifyPlannedRows classifies every roster row of a live (non-void) slot,
// stamping each accounted-for student into seenPlanned so the presence sweep does
// not re-add them.
func (s *service) classifyPlannedRows(
	p slotContext,
	planned []timetable.InstanceStudent,
	presentSet map[int64]bool,
	completed bool,
	careDay map[int64]ports.CareDay,
	seenPlanned map[int64]struct{},
) []mergedEntry {
	entries := []mergedEntry{}
	for _, row := range planned {
		present := presentSet[row.StudentID] || row.Status == timetable.InstanceAttendancePresent
		// A manual attendance correction is a human decision that outranks stale
		// visit evidence. UpdateAttendanceFields stamps ManualStatusAt whenever
		// staff set the row's status by hand, so when an erroneous scan created a
		// visit (row still in presentSet) and staff then corrected the roster to
		// absent or expected, the visit must NOT force Present=true: trust the
		// corrected row status alone. Without this the Ist/Abgleich lists and their
		// counters would report the child "Anwesend" against the explicit human
		// correction (#1565 review pass 12).
		if row.ManualStatusAt != nil {
			present = row.Status == timetable.InstanceAttendancePresent
		}
		// The canonical care-day verdict decides whether an assignment row
		// is a genuine expectation on this date; it folds the frozen
		// not_scheduled marker and the live care-plan derivation into the
		// single answer every reader (planner, roster, cards) shares (#1747).
		verdict := s.rules.RowCareDay(completed, rosterFacts(row), careDay[row.StudentID])
		emitted, markSeen := s.classifyPlannedRow(p, row, present, verdict, careDay[row.StudentID])
		if markSeen {
			seenPlanned[row.StudentID] = struct{}{}
		}
		entries = append(entries, emitted...)
	}
	return entries
}

// appendUnseenPresent emits a present-only row for every student with visit
// evidence that the planned classification did not already account for, and
// reports whether any such row was produced (for the void-plan accounting).
func appendUnseenPresent(p slotContext, presentSet map[int64]bool, seenPlanned map[int64]struct{}) ([]mergedEntry, bool) {
	entries := []mergedEntry{}
	anyPresent := false
	for studentID, present := range presentSet {
		if !present {
			continue
		}
		if _, ok := seenPlanned[studentID]; ok {
			continue
		}
		anyPresent = true
		entries = append(entries, mergedEntry{
			StudentID:  studentID,
			InstanceID: p.inst.ID,
			SlotLabel:  p.slotLabel,
			RoomName:   p.roomName,
			Present:    true,
		})
	}
	return entries, anyPresent
}

// recordDeferredAccounting records a not-yet-started reconciliation slot that
// produced no rows, so the export "Enthalten" summary does not count content
// absent from the document. A deferred slot that retained a present or
// registered-absence row contributed content and therefore counts normally.
func recordDeferredAccounting(result *classday.Result, instanceID int64, deferred, retainedRows bool) {
	if deferred && !retainedRows {
		if result.DeferredSlots == nil {
			result.DeferredSlots = map[int64]struct{}{}
		}
		result.DeferredSlots[instanceID] = struct{}{}
	}
}

// retainDeferredRows keeps the roster rows that must survive a not-yet-started
// reconciliation slot: documented presence and registered all-day sign-offs.
// It records every retained student in seenPlanned and reports whether any row
// was kept for the export "Enthalten" accounting.
//
// A void plan is never a source of "Fehlt": the caller nils `planned` after this
// so no roster row reaches the switch. Two kinds of row still carry real
// information that must survive that suppression:
//
//   - Present evidence. A timetable PATCH records status=present on the roster row
//     without ever attaching an active group, so such a child is absent from the
//     visit-derived presentSet. Retain those rows as unplanned-present evidence so
//     documented attendance still surfaces in Ist and Abgleich (#1565 review
//     pass 1 P2 / pass 2 P1).
//
//   - A registered absence on a DEFERRED (not-yet-started) slot. A child signed
//     off sick/excused/class-trip (ApplyStatusDay stamps the row absent with the
//     substatus) or with a cancelled care day ("Kommt heute nicht") is a valid
//     all-day sign-off, not a not-yet-due no-show. Retain it as "Abgemeldet" —
//     mirroring the pickup-cohort reconciliation, which defers only the unexplained
//     would-be-"Fehlt" child and keeps registered absences — so it shows in the
//     Abgleich and the excused counter before the slot's scheduled start (#1565
//     review pass 1 P2).
func (s *service) retainDeferredRows(
	p slotContext,
	planned []timetable.InstanceStudent,
	careDay map[int64]ports.CareDay,
	seenPlanned map[int64]struct{},
) ([]mergedEntry, bool) {
	entries := []mergedEntry{}
	retained := false
	for _, row := range planned {
		switch row.Status {
		case timetable.InstanceAttendancePresent:
			seenPlanned[row.StudentID] = struct{}{}
			retained = true
			entries = append(entries, mergedEntry{
				StudentID:  row.StudentID,
				InstanceID: p.inst.ID,
				SlotLabel:  p.slotLabel,
				RoomName:   p.roomName,
				Present:    true,
			})
		default:
			substatus, ok := voidPlanRegisteredAbsence(row, careDay[row.StudentID])
			if !ok {
				continue
			}
			seenPlanned[row.StudentID] = struct{}{}
			retained = true
			substatusCopy := substatus
			entries = append(entries, mergedEntry{
				StudentID:        row.StudentID,
				InstanceID:       p.inst.ID,
				SlotLabel:        p.slotLabel,
				RoomName:         p.roomName,
				Planned:          true,
				Present:          false,
				PlannedStatus:    timetable.InstanceAttendanceAbsent,
				PlannedSubstatus: &substatusCopy,
			})
		}
	}
	return entries, retained
}

// classifyPlannedRow maps one roster row of a live (non-void) slot to its merged
// entries and reports whether the student was accounted for (seenPlanned).
// `present` is already corrected for a manual attendance override by the caller;
// `verdict` is the status-gated care-day verdict and `rawCareDay` the raw
// derivation the non-booking / cancellation branches key on.
func (s *service) classifyPlannedRow(
	p slotContext,
	row timetable.InstanceStudent,
	present bool,
	verdict, rawCareDay ports.CareDay,
) ([]mergedEntry, bool) {
	inst := p.inst
	// presentOnly emits a single unplanned-present row (or nothing when absent).
	// Shared by the IsUnplanned, raw not_scheduled and cancelled-but-attended
	// branches, which all surface documented presence as "ungeplant anwesend".
	presentOnly := func() []mergedEntry {
		if !present {
			return nil
		}
		return []mergedEntry{{
			StudentID:  row.StudentID,
			InstanceID: inst.ID,
			SlotLabel:  p.slotLabel,
			RoomName:   p.roomName,
			Present:    true,
		}}
	}
	completed := inst.Status == timetable.InstanceStatusCompleted
	switch {
	case row.IsUnplanned:
		// #1913: the row was created by an observed walk-in visit, not by
		// planning. It is durable presence evidence, never a plan entry —
		// otherwise the Abgleich would relabel the walk-in as "geplant & anwesend".
		return presentOnly(), true
	case !completed && row.ManualStatusAt == nil && rawCareDay == ports.CareDayNotScheduled:
		// #1747/#1565 review: the care plan never booked this child into the OGS
		// on this weekday. Key on the raw care-day verdict, not the status-gated
		// `verdict`, exactly as the cancellation branch below does: a real check-in
		// flips the row to present and AttendanceRowCareDay then reports unknown (a
		// real status tells its own story), so keying on `verdict` here would lose
		// the non-booking and mislabel a bulk-assigned walk-in as "geplant &
		// anwesend" instead of "ungeplant anwesend". The row may read Expected (the
		// not_scheduled marker is only frozen when the block ends), already be
		// present, or carry an absent status a broad sick/excused status day
		// stamped onto a day the child was never scheduled. None of those is a
		// genuine expectation, so none may print as planned/"Fehlt"/"Abgemeldet":
		// drop it and leave the child unseen so a live visit still surfaces them as
		// unplanned-present below (an absence on an unbooked day shows nowhere). A
		// manual override (ManualStatusAt) still wins and is excluded here, and a
		// signed-off (cancelled) booked day is handled next and stays "Abgemeldet".
		//
		// This RAW-verdict branch is confined to UNFINISHED occurrences
		// (`!completed`): the live care-day derivation reads today's recurring
		// arrival/pickup plan, which is not historized, so on a completed
		// historical slot it may have drifted from what was true then. Ending the
		// block already froze the authoritative verdict — not_scheduled onto the
		// children it spared, real absences onto the booked no-shows — so a
		// completed slot must trust that frozen state (handled by the
		// Expected/!verdict.Expected() and default cases below), never the live
		// plan. Without this gate a post-hoc care-plan edit could relabel a
		// then-planned present child as unplanned, or make a then-planned absence
		// vanish, making past Plan/Abgleich exports depend on today's schedule
		// (#1565 review).
		//
		// Emit the present-only row directly rather than deferring to the present
		// loop: on the slot path `present` can come from a timetable PATCH (row
		// status) with no active-group visit, and the present loop only re-surfaces
		// visit-derived presence — deferring would drop such a child entirely
		// instead of showing "ungeplant anwesend". This mirrors the IsUnplanned
		// branch above.
		return presentOnly(), true
	case row.ManualStatusAt == nil && rawCareDay == ports.CareDayCancelled:
		// #1747/#1565 review: a "Kommt heute nicht" cancelled the booked day. The
		// cancellation is care-plan evidence that outlives the row's attendance
		// status, so key on the raw care-day verdict, not the status-gated
		// `verdict`: a real check-in flips the row to present and a completed block
		// flips a no-show to absent, and for both AttendanceRowCareDay reports
		// unknown (a real/finalized status tells its own story). Keying on `verdict`
		// there would lose the cancellation entirely and mislabel the walk-in as
		// "geplant & anwesend" or the no-show as an unexplained "Fehlt". A manual
		// override (ManualStatusAt) still wins and is excluded here, exactly as
		// AttendanceRowCareDay does.
		//
		// Unlike a genuine non-booking, the day WAS booked and the child signed
		// off, so a no-show belongs on the list as "Abgemeldet" (the exact
		// registered-absence shape the pickup path renders), never silently dropped
		// like not_scheduled. If the child attended anyway, that is unplanned
		// presence: emit the present-only row directly here rather than defer to the
		// present loop below. On the slot path `present` can come from a timetable
		// PATCH (row status) with no active-group visit, and that loop only
		// re-surfaces visit-derived presence (presentSet) — deferring would drop
		// such a child from both Ist and Abgleich entirely instead of showing
		// "ungeplant anwesend" (#1565 review pass 2). This mirrors the IsUnplanned
		// and not_scheduled branches, which emit their present-only row inline for
		// the same reason.
		if present {
			return presentOnly(), true
		}
		cancelledSubstatus := string(ports.CareDayCancelled)
		return []mergedEntry{{
			StudentID:        row.StudentID,
			InstanceID:       inst.ID,
			SlotLabel:        p.slotLabel,
			RoomName:         p.roomName,
			Planned:          true,
			Present:          false,
			PlannedStatus:    timetable.InstanceAttendanceAbsent,
			PlannedSubstatus: &cancelledSubstatus,
		}}, true
	case row.Status == timetable.InstanceAttendanceExpected && !verdict.Expected():
		// #1747 non-booking safety net for a COMPLETED block: ending the slot
		// froze the not_scheduled marker into the row, so AttendanceRowCareDay
		// keeps returning not_scheduled from that frozen flag even if the live
		// care plan has since changed (reading the current plan on a finished
		// day would let a later edit relabel it). The live-derivation non-booking
		// on an active block is already handled by the raw care-day case above;
		// here the frozen verdict is authoritative. Not a genuine expectation:
		// skip, and leave the student unseen so a live visit can still surface them
		// as unplanned below.
		return nil, false
	case row.NotScheduled:
		// Unbooked day the system decided anyway (checked in, or an absence
		// written by a status day). The child was never planned for this slot;
		// presence shows as unplanned, an absence on an unbooked day shows nowhere.
		return presentOnly(), true
	default:
		return []mergedEntry{{
			StudentID:        row.StudentID,
			InstanceID:       inst.ID,
			SlotLabel:        p.slotLabel,
			RoomName:         p.roomName,
			Planned:          true,
			Present:          present,
			PlannedStatus:    row.Status,
			PlannedSubstatus: row.Substatus,
		}}, true
	}
}

func instanceMatchesListKind(inst *slotInstance, listKind classday.ListKind) bool {
	if listKind == classday.ListKindNone {
		return true
	}
	if inst == nil || inst.ListKind == nil {
		return false
	}
	return *inst.ListKind == string(listKind)
}

// lookupRoomName resolves a room's display name, cached per build. A missing
// room is non-fatal (room grouping just shows the "Ohne Raum" bucket).
func (s *service) lookupRoomName(ctx context.Context, roomID int64, cache map[int64]string) (string, error) {
	if roomID == 0 {
		return "", nil
	}
	if name, ok := cache[roomID]; ok {
		return name, nil
	}
	room, err := s.rooms.FindRoom(ctx, roomID)
	if err != nil {
		if errors.Is(err, facilities.ErrRoomNotFound) {
			cache[roomID] = ""
			return "", nil
		}
		return "", fmt.Errorf("lookup room %d: %w", roomID, err)
	}
	cache[roomID] = room.Name
	return room.Name, nil
}

// beforeEffectiveArrival reports whether the service clock is still earlier than
// the child's effective arrival instant on the date. The arrival value is a
// wall-clock time, so it is anchored to the calendar day in Berlin exactly as
// instanceTimeRange anchors a slot's start — never trusted as a raw instant. A
// nil arrival time (no schedule for this weekday, or a weekend) yields false:
// with no expected arrival to defer against, the normal merge applies and a
// genuine no-show still surfaces.
func (s *service) beforeEffectiveArrival(date timezone.Date, arrival *time.Time) bool {
	if arrival == nil {
		return false
	}
	at := *arrival
	start := time.Date(
		date.Year(), date.Month(), date.Day(),
		at.Hour(), at.Minute(), at.Second(), at.Nanosecond(),
		timezone.Berlin,
	)
	return s.currentTime().Before(start)
}

func instanceTimeRange(inst *slotInstance) (time.Time, time.Time) {
	start := time.Date(
		inst.Date.Year(), inst.Date.Month(), inst.Date.Day(),
		inst.StartTime.Hour(), inst.StartTime.Minute(), inst.StartTime.Second(), inst.StartTime.Nanosecond(),
		timezone.Berlin,
	)
	end := time.Date(
		inst.Date.Year(), inst.Date.Month(), inst.Date.Day(),
		inst.EndTime.Hour(), inst.EndTime.Minute(), inst.EndTime.Second(), inst.EndTime.Nanosecond(),
		timezone.Berlin,
	)
	return start, end
}

// eligibleOn reports whether a student may be a candidate on the given date.
// Actual attendance wins before enrollment bounds are applied: a child who is
// still present must remain visible even after their planned care ended.
//
// Immediate activation (enrollment.default_activation_mode = "immediate") is the
// deliberate exception: the enrollment decision service creates an already
// 'active' student while keeping enrolled_from at the phase's future
// ServiceStartDate, so the child appears in lists/attendance from today. An
// active status therefore overrides the enrolled_from lower bound — but only
// from today onward: the override lets the child appear for the current and
// future dates before the phase officially starts, it must NOT make the child
// retroactively enrolled for every past date before enrolled_from. Otherwise a
// stale or manually created slot roster (or the /options counts) would show the
// child as planned/missing before their enrollment ever began (#1565 review).
// The enrollment part of that rule lives in userModel.EnrolledOn so the slot
// lists, the day-log rosters and the statistics report cannot drift apart
// (#1565, #2606); presence and alumnus status are decided here because they
// are candidate rules, not enrollment ones.
//
// today is the service clock's calendar day (s.todayDate()), threaded in so
// the immediate-activation cutoff uses the same clock as every other date
// guard in BuildList/ListOptions — deterministic simulations and
// time-controlled tests pin SlotListDependencies.Now, and reading the process clock
// here instead would decide eligibility against a different "today" (#1565
// review).
func eligibleOn(rules ports.Rules, student *peopledirectory.Student, date, today timezone.Date, actuallyPresent bool) (bool, error) {
	if student == nil {
		return false, nil
	}
	if actuallyPresent {
		return true, nil
	}
	if student.Status == peopledirectory.StudentStatusAlumnus {
		return false, nil
	}
	facts, err := studentFacts(student)
	if err != nil {
		return false, err
	}
	return rules.EnrolledOn(facts, date, today), nil
}

// listEligibleStudents returns the cohort candidates shared by the pickup
// builder and options aggregation. actual is supplied by the live pickup
// reader so an open or historical attendance row survives the upper boundary.
//
// The directory lists every non-alumni child of the tenant; the graduates the
// live reader still reports present are fetched by id so documented
// attendance keeps winning over the lifecycle status, exactly as before the
// projection read the owner seams.
func (s *service) listEligibleStudents(
	ctx context.Context, date timezone.Date, actual map[int64]struct{},
) ([]peopledirectory.Student, error) {
	all, err := s.students.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, err
	}
	listed := make(map[int64]struct{}, len(all))
	for _, student := range all {
		listed[student.ID] = struct{}{}
	}
	missing := make([]int64, 0)
	for id := range actual {
		if _, ok := listed[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
		graduates, err := s.students.ListStudentsByID(ctx, missing)
		if err != nil {
			return nil, err
		}
		all = append(all, graduates...)
	}
	today := s.todayDate()
	eligible := make([]peopledirectory.Student, 0, len(all))
	for i := range all {
		student := &all[i]
		_, present := actual[student.ID]
		ok, err := eligibleOn(s.rules, student, date, today, present)
		if err != nil {
			return nil, err
		}
		if ok {
			eligible = append(eligible, *student)
		}
	}
	return eligible, nil
}

// cohortPickupTime returns the HH:MM used to place a student into a Ganztag
// cohort on a date, or "" when the child is not in care that day. A cleared
// same-day exception ("Kommt heute nicht") drops the effective pickup time; a
// cancelled care verdict then falls back to the regular weekly bucket so the
// signed-off child still appears in their cohort. Preview, export, and the
// options availability counts MUST all apply this identical fallback or they
// disagree on cancelled care days (#1565 review).
func (s *service) regularPickupBucket(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) (map[int64]string, error) {
	out, err := s.pickupBaselines.RegularPickupTimes(ctx, studentIDs, date)
	if err != nil {
		return nil, fmt.Errorf("load regular pickup schedules: %w", err)
	}
	if out == nil {
		out = map[int64]string{}
	}
	return out, nil
}

func cohortPickupTime(cancelled bool, effective *time.Time, regular string) string {
	if effective != nil {
		return effective.Format(timeLayout)
	}
	if cancelled {
		return regular
	}
	return ""
}

func includePickupParticipant(careDay ports.CareDay, present bool) bool {
	return careDay != ports.CareDayNotScheduled || present
}

func (s *service) loadPickupCandidates(
	ctx context.Context, date timezone.Date,
) ([]peopledirectory.Student, []int64, map[int64]struct{}, error) {
	attendanceRows, err := s.presence.ListAttendance(ctx, studentpresence.AttendanceFilter{
		FromDate: date.String(), UntilDate: date.String(), StudentOrder: true,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	present := make(map[int64]struct{}, len(attendanceRows))
	for _, row := range attendanceRows {
		present[row.StudentID] = struct{}{}
	}
	students, err := s.listEligibleStudents(ctx, date, present)
	if err != nil {
		return nil, nil, nil, err
	}
	studentIDs := make([]int64, 0, len(students))
	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
	}
	return students, studentIDs, present, nil
}

type pickupEntryInputs struct {
	pickupTimes    map[int64]*time.Time
	arrivalTimes   map[int64]*time.Time
	statusByID     map[int64]string
	partialByID    map[int64]string
	careDays       map[int64]ports.CareDay
	regularByID    map[int64]string
	presentStudent map[int64]struct{}
}

// collectPickupEntries derives the cohort from effective times, dated care
// participation and attendance. Closed attendance rows count historically.
func (s *service) collectPickupEntries(
	ctx context.Context, params listRequest, buckets pickupBucketConfig, result *classday.Result,
) ([]mergedEntry, error) {
	students, studentIDs, present, err := s.loadPickupCandidates(ctx, params.date)
	if err != nil {
		return nil, err
	}
	inputs, err := s.loadPickupEntryInputs(ctx, params, studentIDs, present)
	if err != nil {
		return nil, err
	}
	entries, cohort := s.buildPickupCohortEntries(params, buckets, result.ListLabel, students, inputs)
	return appendUnplannedPickupEntries(entries, cohort, params, buckets, result.ListLabel, inputs), nil
}

func (s *service) loadPickupEntryInputs(
	ctx context.Context, params listRequest, studentIDs []int64, present map[int64]struct{},
) (pickupEntryInputs, error) {
	pickups, arrivals, err := s.loadPickupTiming(ctx, params, studentIDs)
	if err != nil {
		return pickupEntryInputs{}, err
	}
	statuses, partial, err := s.loadPickupAbsences(ctx, studentIDs, params.date)
	if err != nil {
		return pickupEntryInputs{}, err
	}
	careDays, regular, err := s.loadPickupCareEvidence(ctx, studentIDs, params.date)
	if err != nil {
		return pickupEntryInputs{}, err
	}
	return pickupEntryInputs{pickups, arrivals, statuses, partial, careDays, regular, present}, nil
}

func (s *service) loadPickupTiming(
	ctx context.Context, params listRequest, studentIDs []int64,
) (map[int64]*time.Time, map[int64]*time.Time, error) {
	pickups := map[int64]*time.Time{}
	arrivals := map[int64]*time.Time{}
	if len(studentIDs) == 0 {
		return pickups, arrivals, nil
	}
	var err error
	pickups, err = s.effectiveTimes.PickupTimes(ctx, studentIDs, params.date)
	if err != nil {
		return nil, nil, err
	}
	if pickups == nil {
		pickups = map[int64]*time.Time{}
	}
	if params.Source == classday.SourceReconciliation {
		arrivals, err = s.effectiveTimes.ArrivalTimes(ctx, studentIDs, params.date)
		if err != nil {
			return nil, nil, fmt.Errorf("load effective arrival times: %w", err)
		}
		if arrivals == nil {
			arrivals = map[int64]*time.Time{}
		}
	}
	return pickups, arrivals, nil
}

func (s *service) loadPickupAbsences(
	ctx context.Context, studentIDs []int64, date timezone.Date,
) (map[int64]string, map[int64]string, error) {
	statuses := map[int64]string{}
	partial := map[int64]string{}
	if len(studentIDs) == 0 {
		return statuses, partial, nil
	}
	// Active sign-offs plus the scheduler's end-of-day archived rows (source =
	// "end_of_day"), so a child signed off sick/excused all day still renders
	// "Abgemeldet" after the configured status-clear time rather than an
	// unexplained "Fehlt" (#1565 review pass 1).
	statusDays, err := s.carePlan.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		StudentIDs: studentIDs, Date: careplan.Date(date.String()), IncludeEndOfDay: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("load student status days: %w", err)
	}
	for _, day := range statusDays {
		statuses[day.StudentID] = day.Status
	}
	exceptions, err := s.carePlan.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{
		StudentIDs: studentIDs, Date: careplan.Date(date.String()),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("load pickup exceptions for partial absences: %w", err)
	}
	for _, exc := range exceptions {
		if exc.ExcusedFrom != nil {
			partial[exc.StudentID] = timezone.NormalizeWallClock(*exc.ExcusedFrom).Format(timeLayout)
		}
	}
	return statuses, partial, nil
}

func (s *service) loadPickupCareEvidence(
	ctx context.Context, studentIDs []int64, date timezone.Date,
) (map[int64]ports.CareDay, map[int64]string, error) {
	careDays, err := s.careDays.ResolveForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve care days: %w", err)
	}
	regular, err := s.regularPickupBucket(ctx, studentIDs, date)
	if err != nil {
		return nil, nil, err
	}
	return careDays, regular, nil
}

func (s *service) buildPickupCohortEntries(
	params listRequest, buckets pickupBucketConfig, label string,
	students []peopledirectory.Student, inputs pickupEntryInputs,
) ([]mergedEntry, map[int64]struct{}) {
	entries := []mergedEntry{}
	cohort := map[int64]struct{}{}
	for _, student := range students {
		entry, belongs := s.pickupEntryForStudent(params, buckets, label, student.ID, inputs)
		if belongs {
			cohort[student.ID] = struct{}{}
		}
		if entry != nil {
			entries = append(entries, *entry)
		}
	}
	return entries, cohort
}

func (s *service) pickupEntryForStudent(
	params listRequest, buckets pickupBucketConfig, label string, studentID int64, inputs pickupEntryInputs,
) (*mergedEntry, bool) {
	careDay := inputs.careDays[studentID]
	cancelled := careDay == ports.CareDayCancelled
	hhmm := cohortPickupTime(cancelled, inputs.pickupTimes[studentID], inputs.regularByID[studentID])
	if hhmm == "" || !pickupMatchesCohort(params.PickupCohort, hhmm, buckets) {
		return nil, false
	}
	_, present := inputs.presentStudent[studentID]
	if !includePickupParticipant(careDay, present) {
		return nil, false
	}
	if careDay == ports.CareDayNotScheduled || cancelled && present {
		return &mergedEntry{StudentID: studentID, SlotLabel: label, PickupTime: hhmm, Present: true}, true
	}
	status, hasStatus := inputs.statusByID[studentID]
	partial := partialCoversPickup(hhmm, inputs.partialByID[studentID])
	if params.Source == classday.SourceReconciliation && !present && !cancelled && !hasStatus && !partial &&
		s.beforeEffectiveArrival(params.date, inputs.arrivalTimes[studentID]) {
		return nil, true
	}
	return plannedPickupEntry(studentID, label, hhmm, present, cancelled, partial, status, hasStatus), true
}

func plannedPickupEntry(
	studentID int64, label, hhmm string, present, cancelled, partial bool, status string, hasStatus bool,
) *mergedEntry {
	entry := &mergedEntry{StudentID: studentID, SlotLabel: label, PickupTime: hhmm, Planned: true, Present: present}
	if present {
		return entry
	}
	entry.PlannedStatus = timetable.InstanceAttendanceAbsent
	switch {
	case hasStatus:
		entry.PlannedSubstatus = &status
	case cancelled:
		substatus := string(ports.CareDayCancelled)
		entry.PlannedSubstatus = &substatus
	case partial:
		substatus := substatusExcused
		entry.PlannedSubstatus = &substatus
	default:
		entry.PlannedStatus = ""
	}
	return entry
}

func appendUnplannedPickupEntries(
	entries []mergedEntry, cohort map[int64]struct{}, params listRequest,
	buckets pickupBucketConfig, label string, inputs pickupEntryInputs,
) []mergedEntry {
	if params.Source != classday.SourceReconciliation {
		return entries
	}
	for id := range inputs.presentStudent {
		if _, belongs := cohort[id]; belongs {
			continue
		}
		cancelled := inputs.careDays[id] == ports.CareDayCancelled
		resolved := cohortPickupTime(cancelled, inputs.pickupTimes[id], inputs.regularByID[id])
		if pickupInAnyCohort(resolved, buckets) {
			continue
		}
		pickupLabel := ""
		if pickup := inputs.pickupTimes[id]; pickup != nil {
			pickupLabel = pickup.Format(timeLayout)
		}
		entries = append(entries, mergedEntry{
			StudentID: id, SlotLabel: label, PickupTime: pickupLabel, Present: true,
		})
	}
	return entries
}

func pickupMatchesCohort(cohort classday.PickupCohort, hhmm string, buckets pickupBucketConfig) bool {
	switch cohort {
	case classday.PickupCohortShortDay:
		return hhmm <= buckets.ShortCutoff
	case classday.PickupCohortLongDay:
		return hhmm > buckets.ShortCutoff && hhmm <= buckets.LongCutoff
	default:
		return false
	}
}

// pickupInAnyCohort reports whether hhmm places a child into any valid pickup
// cohort (short or long day). A child with such a time is planned in exactly one
// cohort, so a reconciliation for a DIFFERENT cohort must not treat their
// presence as unplanned. Returns false for an empty time (not in care) or a
// pickup past the long-day cutoff (no cohort claims them).
func pickupInAnyCohort(hhmm string, buckets pickupBucketConfig) bool {
	if hhmm == "" {
		return false
	}
	return pickupMatchesCohort(classday.PickupCohortShortDay, hhmm, buckets) ||
		pickupMatchesCohort(classday.PickupCohortLongDay, hhmm, buckets)
}

// enrichEntries filters by source/read access, resolves names/classes/groups,
// and maps to display rows.
func (s *service) enrichEntries(ctx context.Context, entries []mergedEntry, source classday.Source, access studentReadAccess, date timezone.Date) ([]classday.Row, error) {
	// matchesSource is the per-source selection predicate. A Plan list selects
	// rows that are planned, an Ist list rows that are present, an Abgleich keeps
	// everything. It is applied once to build `filtered` and re-applied after the
	// enrollment gate below mutates entries, so the two never diverge.
	matchesSource := func(e mergedEntry) bool {
		switch source {
		case classday.SourcePlanned:
			return e.Planned
		case classday.SourceActual:
			return e.Present
		case classday.SourceReconciliation:
			return true
		default:
			return false
		}
	}
	filtered := make([]mergedEntry, 0, len(entries))
	for _, entry := range entries {
		if matchesSource(entry) {
			filtered = append(filtered, entry)
		}
	}
	if len(filtered) == 0 {
		return []classday.Row{}, nil
	}

	studentIDSet := map[int64]struct{}{}
	studentIDs := []int64{}
	for _, entry := range filtered {
		if _, ok := studentIDSet[entry.StudentID]; !ok {
			studentIDSet[entry.StudentID] = struct{}{}
			studentIDs = append(studentIDs, entry.StudentID)
		}
	}
	studentRows, err := s.students.ListStudentsByID(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	students := make(map[int64]*peopledirectory.Student, len(studentRows))
	for i := range studentRows {
		students[studentRows[i].ID] = &studentRows[i]
	}

	// Date-effective enrollment gate: the lifecycle scheduler inactivates a
	// student by flipping users.students.status only — already-materialized
	// schedule.instance_students rows survive. A *planned* row for a child not
	// enrolled on the requested date must be dropped, or it surfaces as an
	// unexplained "Fehlt" (#1565 review). Documented presence is ground truth
	// and always kept: a real visit/attendance record outranks the enrollment
	// interval, so present rows pass regardless — but the enrollment check has
	// established that the roster's *expectation* is invalid, so a present row
	// for a non-enrolled child must shed its planned classification too, or the
	// Abgleich reports it as planned-and-present and inflates the planned
	// counter. Keep the presence, clear the plan, so it reads as "ungeplant
	// anwesend" (#1565 review pass 2).
	today := s.todayDate()
	enrolled := make([]mergedEntry, 0, len(filtered))
	for _, entry := range filtered {
		student := students[entry.StudentID]
		if student == nil {
			continue
		}
		enrolledOn, err := eligibleOn(s.rules, student, date, today, false)
		if err != nil {
			return nil, err
		}
		if !enrolledOn {
			if !entry.Present {
				continue
			}
			entry.Planned = false
			entry.PlannedStatus = ""
			entry.PlannedSubstatus = nil
			// Clearing the plan can drop the entry below the source predicate it
			// originally satisfied. On a Plan list the row only reached `filtered`
			// because it was planned; now de-planned, keeping it would leak an
			// "ungeplant" row into the planned roster — rendered as Geplant yet
			// excluded from the planned counter (a contradictory row). Re-apply the
			// source filter so a de-planned present row survives only where the list
			// actually selects on presence (Ist/Abgleich), not on a Plan list
			// (#1565 review pass 2).
			if !matchesSource(entry) {
				continue
			}
		}
		enrolled = append(enrolled, entry)
	}
	filtered = enrolled
	if len(filtered) == 0 {
		return []classday.Row{}, nil
	}

	readableStudents := make(map[int64]*peopledirectory.Student, len(students))
	personIDs := make([]int64, 0, len(students))
	groupIDSet := map[int64]struct{}{}
	groupIDs := []int64{}
	for _, entry := range filtered {
		student := students[entry.StudentID]
		if student == nil || !access.canReadGroup(student.GroupID) {
			continue
		}
		if _, ok := readableStudents[student.ID]; ok {
			continue
		}
		readableStudents[student.ID] = student
		personIDs = append(personIDs, student.PersonID)
		if student.GroupID != nil {
			if _, ok := groupIDSet[*student.GroupID]; !ok {
				groupIDSet[*student.GroupID] = struct{}{}
				groupIDs = append(groupIDs, *student.GroupID)
			}
		}
	}
	if len(readableStudents) == 0 {
		return []classday.Row{}, nil
	}
	personRows, err := s.persons.ListPersonsByID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	persons := make(map[int64]*peopledirectory.Person, len(personRows))
	for i := range personRows {
		persons[personRows[i].ID] = &personRows[i]
	}
	groups := map[int64]schoolstructure.Group{}
	if len(groupIDs) > 0 {
		groupRows, err := s.groups.ListGroupsByID(ctx, groupIDs)
		if err != nil {
			return nil, err
		}
		for _, group := range groupRows {
			groups[group.ID] = group
		}
	}

	rows := make([]classday.Row, 0, len(filtered))
	for _, entry := range filtered {
		student := readableStudents[entry.StudentID]
		if student == nil {
			continue
		}
		person := persons[student.PersonID]
		if person == nil {
			continue
		}
		groupName := ""
		if student.GroupID != nil {
			if group, ok := groups[*student.GroupID]; ok {
				groupName = group.Name
			}
		}
		rows = append(rows, classday.Row{
			StudentID:   entry.StudentID,
			Name:        person.FullName(),
			SchoolClass: student.SchoolClass,
			GroupName:   groupName,
			GroupID:     student.GroupID,
			InstanceID:  entry.InstanceID,
			Slot:        entry.SlotLabel,
			RoomName:    entry.RoomName,
			PickupTime:  entry.PickupTime,
			Planned:     entry.Planned,
			Present:     entry.Present,
			Unplanned:   entry.Present && !entry.Planned,
			Excused:     entry.Planned && !entry.Present && signedOffAbsence(entry),
			StatusLabel: statusLabel(entry, source),
		})
	}
	return rows, nil
}

// signedOffAbsence reports whether an absent row carries explicit evidence
// that the absence was registered — sick, excused, a field/class trip, or a
// cancelled care day. A merely non-empty substatus is NOT proof: a `late`
// context (a valid state that can linger after a status change) or `other`
// says nothing about a genuine sign-off. Treating those as "Abgemeldet" would
// move a genuinely missing child out of the Missing counter and suppress a
// safety-relevant missing-child signal, so anything short of registered
// absence evidence falls through to "Fehlt".
func signedOffAbsence(entry mergedEntry) bool {
	if entry.PlannedStatus != timetable.InstanceAttendanceAbsent || entry.PlannedSubstatus == nil {
		return false
	}
	switch *entry.PlannedSubstatus {
	case substatusSick, // instance substatus / status day
		substatusExcused,                   // instance substatus / status day / partial-day excusal
		substatusFieldTrip,                 // instance substatus
		careplan.StudentStatusDayClassTrip, // status day
		string(ports.CareDayCancelled):     // cancelled care day
		return true
	default:
		return false
	}
}

// partialCoversPickup reports whether a partial-day excusal cutoff is at or
// before the child's cohort pickup time. Mirrors ApplyPartialAbsence's
// start_time >= excused_from predicate for slots: earlier pickups remain
// expected; pickups at/after the cutoff are signed off.
func partialCoversPickup(pickupHHMM, excusedFromHHMM string) bool {
	if pickupHHMM == "" || excusedFromHHMM == "" {
		return false
	}
	return pickupHHMM >= excusedFromHHMM
}

// voidPlanRegisteredAbsence reports whether a deferred (not-yet-started) slot's
// roster row is a registered all-day sign-off that must survive the void plan as
// "Abgemeldet" rather than be suppressed like an unexplained not-yet-due no-show.
// It returns the substatus to stamp on the retained entry. Two shapes qualify,
// exactly the ones the normal merge and the pickup cohort render as "Abgemeldet":
//
//   - A status-day sick/excused/class-trip stamp on the row itself: ApplyStatusDay
//     writes status=absent plus the substatus (class trip → "field_trip") and a
//     student_status_day_id, so the evidence rides the row even before the slot
//     starts.
//   - A cancelled care day ("Kommt heute nicht"), which lives in the timeless
//     arrival/pickup exceptions rather than on the row. A manual override
//     (ManualStatusAt) still wins and is excluded here, exactly as the normal
//     merge's cancelled-care-day branch treats it.
//
// Anything else — an expected no-show, a lifecycle absent without a sign-off
// substatus, a not_scheduled non-booking — is not a registered absence and stays
// suppressed so the deferred slot never prints a not-yet-due child as "Fehlt".
func voidPlanRegisteredAbsence(row timetable.InstanceStudent, careDay ports.CareDay) (string, bool) {
	// The care plan never booked this child into the OGS on this weekday
	// (CareDayNotScheduled means a plan exists but does not cover today), yet a
	// broad sick/excused/class-trip status day can still stamp the roster row
	// absent+substatus for a day the child was never expected. On a deferred
	// (not-yet-started) slot that is not a registered absence — the child does
	// not belong on the list at all — so drop it here exactly as the normal
	// merge does at its CareDayNotScheduled branch (classifyPlannedRow); keeping it
	// would inflate the planned and excused lists with unbooked children. A
	// manual per-slot override (ManualStatusAt) is a human decision that wins and
	// is deliberately excluded from this guard, matching that same merge branch.
	// A child with no plan at all resolves to CareDayUnknown (not NotScheduled),
	// so a genuine sign-off on an unplanned child still survives below (#1565
	// review pass 10).
	if row.ManualStatusAt == nil && careDay == ports.CareDayNotScheduled {
		return "", false
	}
	if row.Status == timetable.InstanceAttendanceAbsent && row.Substatus != nil {
		switch *row.Substatus {
		case substatusSick,
			substatusExcused,
			substatusFieldTrip:
			return *row.Substatus, true
		}
	}
	if row.ManualStatusAt == nil && careDay == ports.CareDayCancelled {
		return string(ports.CareDayCancelled), true
	}
	return "", false
}

func statusLabel(entry mergedEntry, source classday.Source) string {
	switch source {
	case classday.SourcePlanned:
		return "Geplant"
	case classday.SourceActual:
		return "Anwesend"
	case classday.SourceReconciliation:
		switch {
		case entry.Present && !entry.Planned:
			return "Ungeplant anwesend"
		case entry.Present:
			return "Anwesend"
		case signedOffAbsence(entry):
			return "Abgemeldet (entschuldigt)"
		default:
			return "Fehlt"
		}
	default:
		return ""
	}
}

func countRows(rows []classday.Row, source classday.Source) classday.Counters {
	// A slot list may include several offerings and the same child can hold one
	// row in each, but these counters are child headcounts — the export subtitle
	// labels them "geplante Kinder" / "anwesende Kinder", not assignment entries.
	// Deduplicate by StudentID per category so a child assigned to two offerings
	// counts once, never twice (#1565 review pass 1 P2). Categories stay
	// independent (as the per-row logic already was): a child present in one slot
	// and missing in another appears in both headcounts, matching the row logic.
	planned := map[int64]struct{}{}
	present := map[int64]struct{}{}
	missing := map[int64]struct{}{}
	excused := map[int64]struct{}{}
	unplanned := map[int64]struct{}{}
	for _, row := range rows {
		if row.Planned {
			planned[row.StudentID] = struct{}{}
		}
		if row.Present {
			present[row.StudentID] = struct{}{}
		}
		switch {
		case row.Planned && !row.Present && row.Excused:
			// Registered sign-off: a justified absence, not an unexplained gap.
			excused[row.StudentID] = struct{}{}
		case row.Planned && !row.Present:
			missing[row.StudentID] = struct{}{}
		}
		if row.Unplanned {
			unplanned[row.StudentID] = struct{}{}
		}
	}
	counters := classday.Counters{
		Planned:   len(planned),
		Present:   len(present),
		Missing:   len(missing),
		Excused:   len(excused),
		Unplanned: len(unplanned),
	}
	// Planned/actual previews intentionally show only their own counter;
	// the merge counters are a reconciliation concept.
	switch source {
	case classday.SourcePlanned:
		return classday.Counters{Planned: counters.Planned}
	case classday.SourceActual:
		return classday.Counters{Present: counters.Present}
	default:
		return counters
	}
}

func (s *service) RenderList(ctx context.Context, requested classday.Params, format listexport.Format) (listexport.File, error) {
	result, err := s.BuildList(ctx, requested)
	if err != nil {
		return listexport.File{}, err
	}
	// BuildList already refused a malformed day, so this parse cannot fail.
	date, err := parseDate(requested.Date)
	if err != nil {
		return listexport.File{}, err
	}
	params := listRequest{Params: requested, date: date}

	// Atomic drift guard: the client echoes the content signature of the preview
	// it verified. If this fresh build no longer matches, live data changed
	// between the client's verification and this render — refuse rather than hand
	// out a file that differs from the approved preview (#1565 review pass 2). An
	// empty ExpectedSignature (older client or a direct render) skips the guard.
	if params.ExpectedSignature != "" && result.Signature != params.ExpectedSignature {
		return listexport.File{}, classday.ErrListDrifted
	}

	pickupBased := params.Target == classday.TargetPickupCohort
	columns := []listexport.Column{
		{ID: listexport.ColumnName, Label: "Name"},
		{ID: listexport.ColumnSchoolClass, Label: "Klasse"},
		{ID: listexport.ColumnGroup, Label: "Gruppe"},
		{ID: listexport.ColumnSlot, Label: "Angebot / Zeitraum"},
	}
	if pickupBased {
		columns = append(columns, listexport.Column{ID: listexport.ColumnPlannedPickup, Label: "Geplante Abholung"})
	}
	columns = append(columns, listexport.Column{ID: listexport.ColumnPresenceStatus, Label: "Status"})

	// The PDF renderer treats a Row with GroupTitle as a pure section marker
	// and drops its Values (pdf_design.go paginate). Emit one marker row per
	// section followed by plain value rows — the same shape the grouped
	// student exports produce. Rows arrive sorted by GroupTitle (sortRows),
	// so sections are contiguous.
	docRows := make([]listexport.Row, 0, len(result.Rows))
	currentGroup := ""
	for _, row := range result.Rows {
		if row.GroupTitle != "" && row.GroupTitle != currentGroup {
			currentGroup = row.GroupTitle
			docRows = append(docRows, listexport.Row{GroupTitle: row.GroupTitle})
		}
		values := map[listexport.ColumnID]string{
			listexport.ColumnName:           row.Name,
			listexport.ColumnSchoolClass:    row.SchoolClass,
			listexport.ColumnGroup:          row.GroupName,
			listexport.ColumnSlot:           row.Slot,
			listexport.ColumnPresenceStatus: row.StatusLabel,
		}
		if pickupBased {
			values[listexport.ColumnPlannedPickup] = row.PickupTime
		}
		docRows = append(docRows, listexport.Row{Values: values})
	}

	doc := listexport.Document{
		Title:       documentTitle(result),
		Subtitle:    subtitle(result),
		GeneratedAt: time.Now(),
		Filters:     exportFilters(params, result),
		Columns:     columns,
		Rows:        docRows,
		Footer:      confidentialityNote,
	}

	filename := exportFilename(params, result)
	return s.listExport.Render(doc, format, filename)
}

func documentTitle(result *classday.Result) string {
	return fmt.Sprintf("Tagesliste %s – %s", result.Source.Label(), result.ListLabel)
}

func exportFilename(params listRequest, result *classday.Result) string {
	return fmt.Sprintf("Tagesliste %s %s %s", params.Source.Label(), result.ListLabel, params.Date.String())
}

// exportFilters builds the header lines stamped on the printed export.
func exportFilters(params listRequest, result *classday.Result) []string {
	filters := []string{
		"Datum: " + params.date.Format("02.01.2006"),
		"Datenbasis: " + params.Source.Label() + " – " + result.Provenance,
		"Enthalten: " + includedSummary(params, result),
	}
	if names := selectedGroupNames(params, result); len(names) > 0 {
		filters = append(filters, "Gruppen: "+strings.Join(names, ", "))
	}
	if len(params.Classes) > 0 {
		filters = append(filters, "Klassen: "+strings.Join(params.Classes, ", "))
	}
	if params.GroupBy != classday.GroupByNone {
		filters = append(filters, "Gruppiert nach: "+params.GroupBy.Label())
	}
	return filters
}

// selectedGroupNames maps the selected education-group IDs to their display
// names via the options the build already resolved, so a filtered export
// discloses the restriction instead of looking like the full cohort.
// result.Groups is derived from the day's rows (availableOptions(allRows)), so a
// saved selection that references a group with no rows for this date/source is
// absent from it. Such an ID is disclosed via an explicit "Gruppe #<id>" marker
// rather than dropped — otherwise an empty or partial confidential export would
// print as if it were unfiltered (#1565 review pass 2).
func selectedGroupNames(params listRequest, result *classday.Result) []string {
	if len(params.GroupIDs) == 0 {
		return nil
	}
	byID := make(map[int64]string, len(result.Groups))
	for _, group := range result.Groups {
		byID[group.ID] = group.Name
	}
	names := make([]string, 0, len(params.GroupIDs))
	for _, id := range params.GroupIDs {
		if name := byID[id]; name != "" {
			names = append(names, name)
			continue
		}
		names = append(names, fmt.Sprintf("Gruppe #%d", id))
	}
	sort.Strings(names)
	return names
}

func includedSummary(params listRequest, result *classday.Result) string {
	if params.Target == classday.TargetPickupCohort {
		return result.ListLabel
	}
	count := summarySlotCount(params, result)
	if params.ListKind != classday.ListKindNone {
		return fmt.Sprintf("%s (%d %s)", result.ListLabel, count, plural(count, "Termin", "Termine"))
	}
	return fmt.Sprintf("%d %s", count, plural(count, "Angebot", "Angebote"))
}

// summarySlotCount is the number of slots the export actually contains. When an
// explicit instance selection is present (InstanceIDsSet, even for a classified
// list_kind) the count is restricted to the selected slots — so a filtered
// export reports its real subset instead of claiming the whole cohort (#1565
// review). result.Slots is always the full set of matching slots, so the
// selection has to be re-applied here. Cancelled slots are excluded upstream and
// ignored defensively here; deferred slots (a not-yet-started reconciliation
// occurrence excluded from the merge in collectSlotEntries) are dropped in both
// branches when they yield no export rows, so the header cannot claim content the
// export does not contain (#1565).
func summarySlotCount(params listRequest, result *classday.Result) int {
	all := !params.InstanceIDsSet && len(params.InstanceIDs) == 0
	selected := int64Set(params.InstanceIDs)
	count := 0
	for _, slot := range result.Slots {
		if slot.Status == string(timetable.InstanceStatusCancelled) {
			continue
		}
		if _, deferred := result.DeferredSlots[slot.InstanceID]; deferred {
			continue
		}
		if all || selected[slot.InstanceID] {
			count++
		}
	}
	return count
}

func subtitle(result *classday.Result) string {
	switch result.Source {
	case classday.SourcePlanned:
		return fmt.Sprintf("%d geplante Kinder", result.Counters.Planned)
	case classday.SourceActual:
		return fmt.Sprintf("%d anwesende Kinder", result.Counters.Present)
	default:
		return fmt.Sprintf(
			"%d geplant, %d anwesend, %d fehlend, %d abgemeldet, %d ungeplant",
			result.Counters.Planned,
			result.Counters.Present,
			result.Counters.Missing,
			result.Counters.Excused,
			result.Counters.Unplanned,
		)
	}
}

func plural(count int, singular string, pluralValue string) string {
	if count == 1 {
		return singular
	}
	return pluralValue
}
