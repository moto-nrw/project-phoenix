// Package reminders computes visual-only staff reminders (issue #1457):
// upcoming pickups, overdue pickups, and activity starts. There is no sound and
// no push — the data is derived on request from schedules already in the system
// and rendered on the staff "Erinnerungen" page. All thresholds (which types
// are on, lead-time minutes) are tenant settings resolved at request time.
package reminders

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// Reminder event types.
const (
	TypePickupUpcoming  = "pickup_upcoming"
	TypePickupOverdue   = "pickup_overdue"
	TypeActivityStart   = "activity_start"
	TypeActivityOverdue = "activity_overdue"
)

// Reminder is a single visual reminder shown to staff.
type Reminder struct {
	Type string `json:"type"`
	// Entity IDs are serialized as strings to honor the repo's int64→string API
	// contract (JS loses precision on int64 as a JSON number). Exactly one is set
	// according to Type and gives frontend lists a stable reconciliation key.
	StudentID          *string `json:"student_id,omitempty"`
	ActivityInstanceID *string `json:"activity_instance_id,omitempty"`
	Title              string  `json:"title"`              // student name OR activity title
	Subtitle           string  `json:"subtitle,omitempty"` // school class or room — optional
	DueTime            string  `json:"due_time"`           // "HH:MM" of the relevant time
	MinutesAway        int     `json:"minutes_away"`       // negative when overdue
}

// Result is the computed reminder list plus a convenience count for the badge.
// Enabled reports whether the tenant has switched on at least one reminder
// type — the header bell uses it to show/hide itself (and the /reminders page
// link it exposes) regardless of whether anything is currently due.
type Result struct {
	Reminders []Reminder `json:"reminders"`
	Count     int        `json:"count"`
	Enabled   bool       `json:"enabled"`
	// NextChangeAt is the wall-clock "HH:MM" of the soonest future moment at
	// which this list would change purely by time passing — the next pickup or
	// activity entering/leaving its window. The frontend schedules a timer to
	// exactly this time to refetch, so a reminder appears on its threshold
	// instead of only on the next fixed poll. Omitted when nothing time-based is
	// pending (the poll then remains the only cadence). Data-driven changes
	// (check-ins, edits) are still delivered via the SSE-stale event, not this.
	NextChangeAt string `json:"next_change_at,omitempty"`
}

// Scope describes whose reminders to compute. Admins see all present children
// and all activities; caregivers see only the children currently in the rooms
// they supervise and the activities in those rooms (issue #1457 decision).
type Scope struct {
	IsAdmin bool
	StaffID int64
}

// Service is the reminder computation entry point.
type Service interface {
	Compute(ctx context.Context, scope Scope) (*Result, error)
}

type settingsResolver interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

type attendanceReader interface {
	ListOpenStudentIDsForDate(ctx context.Context, date timezone.Date) ([]int64, error)
}

type pickupReader interface {
	GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*scheduleService.EffectivePickupTime, error)
}

type instanceReader interface {
	FindByTenantAndDate(ctx context.Context, date timezone.Date) ([]*scheduleModel.ActivityInstance, error)
}

type roomReader interface {
	FindByIDs(ctx context.Context, ids []int64) ([]*facilitiesModel.Room, error)
}

type studentReader interface {
	// FindReadScopeByIDs returns only the id/group_id/person_id/school_class
	// projection this service needs — read-access gating plus name display. It
	// deliberately avoids the repository's full FindByIDs hydration (weekday
	// bus-day / departure jsonb + an information_schema probe), which would run on
	// every 60s header poll for the whole present population and buys this service
	// nothing.
	FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*userModel.Student, error)
}

type personReader interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModel.Person, error)
}

type supervisionReader interface {
	GetStaffActiveSupervisions(ctx context.Context, staffID int64) ([]*activeModel.GroupSupervisor, error)
	GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*activeModel.Group, error)
	ListStudentsPresentInRoom(ctx context.Context, roomID int64) ([]int64, error)
}

// groupReader lists the education groups the current caller supervises. It
// mirrors usercontext.UserContextService.GetMyGroups so the pickup read-access
// gate (gdpr.student_data_scope) can be enforced without importing the handler
// layer.
type groupReader interface {
	GetMyGroups(ctx context.Context) ([]*educationModel.Group, error)
}

// Dependencies wires the readers the service needs. They mirror existing
// services/repositories so no new query construction lives here.
type Dependencies struct {
	Settings    settingsResolver
	Attendance  attendanceReader
	Pickup      pickupReader
	Instance    instanceReader
	Room        roomReader
	Student     studentReader
	Person      personReader
	Supervision supervisionReader
	Groups      groupReader
	Logger      *slog.Logger
}

type service struct {
	Dependencies
}

// NewService builds the reminder service.
func NewService(deps Dependencies) Service {
	if deps.Room == nil {
		panic("reminders.NewService: Room is required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &service{Dependencies: deps}
}

func (s *service) Compute(ctx context.Context, scope Scope) (*Result, error) {
	empty := &Result{Reminders: []Reminder{}}
	if s.Settings == nil {
		return empty, nil
	}

	// A resolution failure (DB/RLS/tenant-tx error) must surface, not be treated
	// as "disabled" — otherwise a broken config read looks like a healthy empty
	// result and silently hides reminders the tenant switched on.
	pickupUpcoming, err := s.Settings.ResolveBool(ctx, configModel.KeyRemindersPickupUpcomingEnabled)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", configModel.KeyRemindersPickupUpcomingEnabled, err)
	}
	pickupOverdue, err := s.Settings.ResolveBool(ctx, configModel.KeyRemindersPickupOverdueEnabled)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", configModel.KeyRemindersPickupOverdueEnabled, err)
	}
	activityStart, err := s.Settings.ResolveBool(ctx, configModel.KeyRemindersActivityStartEnabled)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", configModel.KeyRemindersActivityStartEnabled, err)
	}
	activityOverdue, err := s.Settings.ResolveBool(ctx, configModel.KeyRemindersActivityOverdueEnabled)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", configModel.KeyRemindersActivityOverdueEnabled, err)
	}

	// Default-off: nothing enabled means no work and no data exposure.
	if !pickupUpcoming && !pickupOverdue && !activityStart && !activityOverdue {
		return empty, nil
	}

	today := timezone.TodayDate()
	nowMin := minutesOfDay(timezone.Now())

	reminders := make([]Reminder, 0)
	// -1 = no future time-based change pending. Both branches feed their soonest
	// boundary in; the earliest across pickups and activities wins.
	nextChange := -1

	// Pickup and activity scopes are resolved independently so an activity-only
	// tenant never pays for (or fails on) student-presence resolution, and a
	// pickup-only tenant never resolves the room filter.
	if pickupUpcoming || pickupOverdue {
		studentIDs, serr := s.pickupScopeStudentIDs(ctx, scope, today)
		if serr != nil {
			return nil, serr
		}
		lead, lerr := s.leadMinutes(ctx, configModel.KeyRemindersPickupUpcomingLeadMinutes)
		if lerr != nil {
			return nil, lerr
		}
		pickupReminders, pickupNext, perr := s.pickupReminders(ctx, scope, studentIDs, today, nowMin, lead, pickupUpcoming, pickupOverdue)
		if perr != nil {
			return nil, perr
		}
		reminders = append(reminders, pickupReminders...)
		nextChange = minFuture(nextChange, pickupNext)
	}

	if activityStart || activityOverdue {
		// Admins see all activities (nil filter); caregivers are limited to the
		// rooms they supervise.
		var roomIDs []int64
		if !scope.IsAdmin {
			rooms, rerr := s.supervisedRoomIDs(ctx, scope)
			if rerr != nil {
				return nil, rerr
			}
			roomIDs = rooms
		}
		lead, lerr := s.leadMinutes(ctx, configModel.KeyRemindersActivityStartLeadMinutes)
		if lerr != nil {
			return nil, lerr
		}
		overdueThreshold, oerr := s.overdueThresholdMinutes(ctx)
		if oerr != nil {
			return nil, oerr
		}
		activityReminders, activityNext, aerr := s.activityReminders(ctx, scope, roomIDs, today, nowMin, lead, overdueThreshold, activityStart, activityOverdue)
		if aerr != nil {
			return nil, aerr
		}
		reminders = append(reminders, activityReminders...)
		nextChange = minFuture(nextChange, activityNext)
	}

	// Most urgent first (overdue is negative, soonest upcoming next).
	sort.SliceStable(reminders, func(i, j int) bool {
		return reminders[i].MinutesAway < reminders[j].MinutesAway
	})

	result := &Result{Reminders: reminders, Count: len(reminders), Enabled: true}
	if nextChange >= 0 {
		result.NextChangeAt = formatMinutes(nextChange)
	}
	return result, nil
}

// pickupScopeStudentIDs returns the IDs of currently present students whose
// pickups are in scope. This is presence scope only — read access
// (gdpr.student_data_scope) is enforced later in pickupReminders, once the much
// smaller "actually due" set is known.
//
//   - Admins: all present students (open attendance rows), regardless of mode.
//   - Caregivers, detailed mode: students currently present in a supervised
//     room, read from active.visits.
//   - Caregivers, binary mode: binary-mode tenants intentionally no-op
//     CreateVisit and track presence only in active.attendance, so there are no
//     room-scoped visit rows to read (the room-based path would silently return
//     empty even when checked-in students have due pickups). Fall back to all
//     present students from attendance and let the read predicate in
//     pickupReminders lock the set down to the caregiver's own education groups.
func (s *service) pickupScopeStudentIDs(ctx context.Context, scope Scope, today timezone.Date) ([]int64, error) {
	if scope.IsAdmin {
		return s.presentStudentIDsFromAttendance(ctx, today)
	}

	binary, err := s.binaryMode(ctx)
	if err != nil {
		return nil, err
	}
	if binary {
		return s.presentStudentIDsFromAttendance(ctx, today)
	}

	roomIDs, err := s.supervisedRoomIDs(ctx, scope)
	if err != nil {
		return nil, err
	}
	return s.presentStudentsInRooms(ctx, roomIDs)
}

// presentStudentIDsFromAttendance lists all students with an open attendance
// row today. Used for admins (all present children) and for caregivers in
// binary mode, where room-scoped visit rows do not exist.
func (s *service) presentStudentIDsFromAttendance(ctx context.Context, today timezone.Date) ([]int64, error) {
	if s.Attendance == nil {
		return nil, nil
	}
	return s.Attendance.ListOpenStudentIDsForDate(ctx, today)
}

// binaryMode reports whether the tenant runs the binary presence mode, in which
// CreateVisit is a no-op and presence lives in active.attendance rather than
// active.visits. A settings resolution error is surfaced, consistent with the
// rest of the service — a broken read must not silently pick a presence source.
func (s *service) binaryMode(ctx context.Context) (bool, error) {
	if s.Settings == nil {
		return false, nil
	}
	v, err := s.Settings.ResolveString(ctx, configModel.KeyPresenceMode)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", configModel.KeyPresenceMode, err)
	}
	return v == configModel.PresenceModeBinary, nil
}

// supervisedRoomIDs returns the room IDs the caregiver currently supervises.
func (s *service) supervisedRoomIDs(ctx context.Context, scope Scope) ([]int64, error) {
	if s.Supervision == nil {
		return nil, nil
	}

	supervisions, err := s.Supervision.GetStaffActiveSupervisions(ctx, scope.StaffID)
	if err != nil {
		return nil, err
	}
	groupIDs := make([]int64, 0, len(supervisions))
	for _, sup := range supervisions {
		if sup != nil {
			groupIDs = append(groupIDs, sup.GroupID)
		}
	}
	if len(groupIDs) == 0 {
		return nil, nil
	}

	groups, err := s.Supervision.GetActiveGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	roomSet := make(map[int64]struct{}, len(groups))
	for _, g := range groups {
		if g != nil {
			roomSet[g.RoomID] = struct{}{}
		}
	}
	return slices.Collect(maps.Keys(roomSet)), nil
}

// presentStudentsInRooms returns the deduplicated IDs of students currently
// present in any of the given rooms.
func (s *service) presentStudentsInRooms(ctx context.Context, roomIDs []int64) ([]int64, error) {
	if s.Supervision == nil || len(roomIDs) == 0 {
		return nil, nil
	}
	studentSet := make(map[int64]struct{})
	for _, roomID := range roomIDs {
		present, err := s.Supervision.ListStudentsPresentInRoom(ctx, roomID)
		if err != nil {
			return nil, err
		}
		for _, id := range present {
			studentSet[id] = struct{}{}
		}
	}
	return slices.Collect(maps.Keys(studentSet)), nil
}

func (s *service) pickupReminders(ctx context.Context, scope Scope, studentIDs []int64, today timezone.Date, nowMin, lead int, upcoming, overdue bool) ([]Reminder, int, error) {
	nextChange := -1
	if len(studentIDs) == 0 || s.Pickup == nil {
		return nil, nextChange, nil
	}
	times, err := s.Pickup.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, today)
	if err != nil {
		return nil, nextChange, err
	}

	// Apply the read-access gate BEFORE deriving anything from the pickup times,
	// including the next-change boundaries. studentIDs is presence-scoped only and
	// may contain children the caller may not read (binary mode's all-present
	// list, or a non-supervised child sharing a supervised room). Computing a
	// boundary from an unreadable child's pickup would leak that child's exact
	// future pickup minute through next_change_at even though no reminder for them
	// is ever returned — so only students that pass the same gdpr.student_data_scope
	// gate as the emitted reminders may influence this response at all. Names are
	// still hydrated only for the due subset below (a large school may have many
	// present students but only a handful actually within the window, and the
	// header polls every 60s), so the wasteful person lookups stay bounded.
	//
	// This gate necessarily loads every present student (the read predicate needs
	// each Student's group to decide readability, and a boundary is tracked for
	// EVERY readable student, not just the due ones), so the read below scales with
	// the present population rather than the due subset. That is a deliberate
	// trade-off for a correct next_change_at, kept cheap on purpose: readableStudents
	// uses FindReadScopeByIDs — a single primary-key IN-list projection of
	// id/group_id/person_id/school_class only, with NONE of FindByIDs' weekday
	// bus-day hydration or information_schema probing — so a 60s poll over the whole
	// present population stays a small-row lookup. The expensive part — person-name
	// hydration — remains scoped to the due subset.
	readable, err := s.readableStudents(ctx, scope, studentIDs)
	if err != nil {
		return nil, nextChange, err
	}
	if len(readable) == 0 {
		return nil, nextChange, nil
	}

	type duePickup struct {
		id        int64
		pickupMin int
		diff      int
		isOverdue bool
	}
	dues := make([]duePickup, 0)
	for _, id := range studentIDs {
		if _, ok := readable[id]; !ok {
			// Not readable under gdpr.student_data_scope — must contribute neither
			// a reminder nor a next-change boundary.
			continue
		}
		effective := times[id]
		if effective == nil || effective.PickupTime == nil {
			continue
		}
		pickupMin := minutesOfDay(*effective.PickupTime)

		// Next-change boundaries are tracked for EVERY readable student's pickup,
		// not just the currently-due ones: a pickup still outside its lead window
		// contributes the moment it will enter (pickupMin-lead) and the moment it
		// flips upcoming->overdue. The flip happens the first minute pickupMin is
		// in the past — pickupMin+1, NOT pickupMin: at exactly pickupMin the diff
		// is 0, which is still "upcoming" (and, when only upcoming is enabled, the
		// upcoming reminder itself only disappears at pickupMin+1). Scheduling
		// pickupMin would refetch a minute early (nothing changed yet) and, since
		// that boundary is then no longer in the future, drop the real pickupMin+1
		// boundary — leaving the overdue reminder to surface only on the next 60s
		// poll. This is what lets the frontend time the first appearance of a
		// reminder that isn't in this response yet.
		if upcoming {
			nextChange = minFuture(nextChange, futureBoundary(pickupMin-lead, nowMin))
		}
		if upcoming || overdue {
			nextChange = minFuture(nextChange, futureBoundary(pickupMin+1, nowMin))
		}

		diff := pickupMin - nowMin
		switch {
		case diff < 0 && overdue:
			dues = append(dues, duePickup{id: id, pickupMin: pickupMin, diff: diff, isOverdue: true})
		case diff >= 0 && diff <= lead && upcoming:
			dues = append(dues, duePickup{id: id, pickupMin: pickupMin, diff: diff, isOverdue: false})
		}
	}
	if len(dues) == 0 {
		return nil, nextChange, nil
	}

	dueIDs := make([]int64, len(dues))
	for i, d := range dues {
		dueIDs[i] = d.id
	}

	// Hydrate display names for the due students only (all already read-gated).
	infos, err := s.hydrateNames(ctx, readable, dueIDs)
	if err != nil {
		return nil, nextChange, err
	}

	out := make([]Reminder, 0, len(dues))
	for _, d := range dues {
		// dueIDs is a subset of the read-gated `readable` set and hydrateNames
		// populates an entry for every one of them, so infos[d.id] is always
		// present here — no missing-key guard is needed.
		info := infos[d.id]
		if info.name == "" {
			// The student passed the read gate but its person record could not be
			// resolved (a not-found row rather than a read error — e.g. the person
			// was soft-deleted mid-day while the student still has an open
			// attendance row). A reminder with a blank title is an unidentifiable
			// card, so skip it: this upholds the same "no unusable empty-title
			// reminders" guarantee the person-read-error path enforces by failing.
			if s.Logger != nil {
				s.Logger.Debug("skipping pickup reminder with unresolved name", "student_id", d.id)
			}
			continue
		}
		reminderType := TypePickupUpcoming
		if d.isOverdue {
			reminderType = TypePickupOverdue
		}
		out = append(out, Reminder{
			Type:        reminderType,
			StudentID:   int64String(d.id),
			Title:       info.name,
			Subtitle:    info.class,
			DueTime:     formatMinutes(d.pickupMin),
			MinutesAway: d.diff,
		})
	}
	return out, nextChange, nil
}

func (s *service) activityReminders(ctx context.Context, scope Scope, roomIDs []int64, today timezone.Date, nowMin, lead, overdueThreshold int, upcoming, overdue bool) ([]Reminder, int, error) {
	nextChange := -1
	if s.Instance == nil {
		return nil, nextChange, nil
	}
	instances, err := s.Instance.FindByTenantAndDate(ctx, today)
	if err != nil {
		return nil, nextChange, err
	}

	var roomFilter map[int64]struct{}
	if !scope.IsAdmin {
		roomFilter = make(map[int64]struct{}, len(roomIDs))
		for _, id := range roomIDs {
			roomFilter[id] = struct{}{}
		}
	}

	schulhofRoomIDs := make(map[int64]struct{})
	if overdue {
		if s.Room == nil {
			return nil, nextChange, errors.New("activity reminder room reader is not configured")
		}
		uniqueRoomIDs := make(map[int64]struct{})
		for _, inst := range instances {
			if inst != nil && inst.Status == scheduleModel.InstanceStatusPlanned {
				uniqueRoomIDs[inst.RoomID] = struct{}{}
			}
		}
		roomIDsToLoad := make([]int64, 0, len(uniqueRoomIDs))
		for roomID := range uniqueRoomIDs {
			roomIDsToLoad = append(roomIDsToLoad, roomID)
		}
		rooms, roomErr := s.Room.FindByIDs(ctx, roomIDsToLoad)
		if roomErr != nil {
			return nil, nextChange, fmt.Errorf("load activity reminder rooms: %w", roomErr)
		}
		resolvedRoomIDs := make(map[int64]struct{}, len(rooms))
		for _, room := range rooms {
			if room == nil {
				continue
			}
			resolvedRoomIDs[room.ID] = struct{}{}
			if room.Name == constants.SchulhofRoomName {
				schulhofRoomIDs[room.ID] = struct{}{}
			}
		}
		for roomID := range uniqueRoomIDs {
			if _, found := resolvedRoomIDs[roomID]; !found {
				return nil, nextChange, fmt.Errorf("load activity reminder rooms: room %d not found", roomID)
			}
		}
	}

	out := make([]Reminder, 0)
	for _, inst := range instances {
		// Only planned instances are relevant: started/completed/cancelled rows
		// are neither "starting soon" nor "not started in time".
		if inst == nil || inst.Status != scheduleModel.InstanceStatusPlanned {
			continue
		}
		if roomFilter != nil {
			if _, ok := roomFilter[inst.RoomID]; !ok {
				continue
			}
		}
		startMin := minutesOfDay(inst.StartTime)
		endMin := minutesOfDay(inst.EndTime)
		diff := startMin - nowMin
		_, isSchulhof := schulhofRoomIDs[inst.RoomID]

		// Track the boundaries at which this instance's reminder state flips, so
		// the frontend can refetch exactly then instead of waiting for the poll:
		//   upcoming: enters the window at startMin-lead, leaves it at startMin+1
		//   overdue:  appears at startMin+overdueThreshold, disappears at endMin
		// The upcoming exit is startMin+1, NOT startMin: at exactly startMin the
		// diff is 0, which is still "upcoming" (diff >= 0); the reminder only
		// disappears the first minute startMin is in the past. Scheduling startMin
		// would refetch a minute early (nothing changed) and drop the real
		// startMin+1 boundary — same reasoning as the pickup flip below.
		if upcoming {
			nextChange = minFuture(nextChange, futureBoundary(startMin-lead, nowMin))
			nextChange = minFuture(nextChange, futureBoundary(startMin+1, nowMin))
		}
		if overdue && !isSchulhof {
			overdueStart := startMin + overdueThreshold
			if overdueStart < endMin {
				nextChange = minFuture(nextChange, futureBoundary(overdueStart, nowMin))
				nextChange = minFuture(nextChange, futureBoundary(endMin, nowMin))
			}
		}

		switch {
		case upcoming && diff >= 0 && diff <= lead:
			out = append(out, Reminder{
				Type:               TypeActivityStart,
				ActivityInstanceID: int64String(inst.ID),
				Title:              inst.Title,
				DueTime:            formatMinutes(startMin),
				MinutesAway:        diff,
			})
		// Overdue: planned, started late by at least the threshold, and the
		// slot is not over yet (after end_time a reminder is pointless).
		case overdue && !isSchulhof && diff < 0 && -diff >= overdueThreshold && nowMin < endMin:
			out = append(out, Reminder{
				Type:               TypeActivityOverdue,
				ActivityInstanceID: int64String(inst.ID),
				Title:              inst.Title,
				DueTime:            formatMinutes(startMin),
				MinutesAway:        diff,
			})
		}
	}
	return out, nextChange, nil
}

type studentNameInfo struct {
	name  string
	class string
}

// readableStudents loads the given students and returns only those the caller is
// allowed to read. It enforces the same gdpr.student_data_scope gate the
// pickup-schedule endpoints use: room presence alone does not grant visibility
// of a student's pickup data — a caregiver must supervise the student's
// education group (unless the tenant runs the all_staff scope, or the caller is
// an admin).
//
// A DB/RLS lookup error or a settings-resolution error is propagated rather than
// treated as no-access, so a broken read fails the request instead of silently
// hiding (or exposing) reminders.
func (s *service) readableStudents(ctx context.Context, scope Scope, ids []int64) (map[int64]*userModel.Student, error) {
	if s.Student == nil {
		// Without a student reader we can neither verify read access nor build a
		// title. Fail closed (return nothing) rather than expose unverified data.
		return map[int64]*userModel.Student{}, nil
	}

	students, err := s.Student.FindReadScopeByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load students: %w", err)
	}

	allowed, err := s.pickupReadPredicate(ctx, scope)
	if err != nil {
		return nil, err
	}

	readable := make(map[int64]*userModel.Student, len(students))
	for id, st := range students {
		if st == nil || !allowed(st) {
			continue
		}
		readable[id] = st
	}
	return readable, nil
}

// hydrateNames builds the display name + school class for the given (already
// read-gated) students. Only the due subset is hydrated: the pickup-reminder API
// contract promises a student name as the title, so a failed person read must
// fail the request rather than emit unusable reminders with empty titles.
func (s *service) hydrateNames(ctx context.Context, readable map[int64]*userModel.Student, ids []int64) (map[int64]studentNameInfo, error) {
	result := make(map[int64]studentNameInfo, len(ids))

	personIDs := make([]int64, 0, len(ids))
	for _, id := range ids {
		if st := readable[id]; st != nil {
			personIDs = append(personIDs, st.PersonID)
		}
	}

	var persons map[int64]*userModel.Person
	if s.Person != nil && len(personIDs) > 0 {
		var err error
		persons, err = s.Person.FindByIDs(ctx, personIDs)
		if err != nil {
			return nil, fmt.Errorf("load persons for student names: %w", err)
		}
	}

	for _, id := range ids {
		st := readable[id]
		if st == nil {
			continue
		}
		info := studentNameInfo{class: st.SchoolClass}
		if persons != nil {
			if p := persons[st.PersonID]; p != nil {
				info.name = p.GetFullName()
			}
		}
		result[id] = info
	}
	return result, nil
}

// pickupReadPredicate returns a per-student read-access test mirroring
// authorize.CanReadStudent for the pickup read path:
//   - Admins and the gdpr.student_data_scope=all_staff scope may read every
//     present student.
//   - Otherwise (group_supervisors_only), only students in an education group
//     the caregiver supervises are readable. Room presence alone is not enough.
//
// A settings resolution error is surfaced, consistent with the rest of the
// service — a broken config read must not silently widen or narrow visibility.
func (s *service) pickupReadPredicate(ctx context.Context, scope Scope) (func(*userModel.Student) bool, error) {
	if scope.IsAdmin {
		return func(*userModel.Student) bool { return true }, nil
	}

	scopeVal := configModel.StudentDataScopeGroupSupervisorsOnly
	if s.Settings != nil {
		// ResolveString returns the registry default (group_supervisors_only)
		// when the tenant has no override, so there is no env-var fallback to
		// consider here.
		v, err := s.Settings.ResolveString(ctx, configModel.KeyStudentDataScope)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", configModel.KeyStudentDataScope, err)
		}
		if v != "" {
			scopeVal = v
		}
	}
	if scopeVal == configModel.StudentDataScopeAllStaff {
		return func(*userModel.Student) bool { return true }, nil
	}

	groupSet := make(map[int64]struct{})
	if s.Groups != nil {
		eduGroups, err := s.Groups.GetMyGroups(ctx)
		if err != nil {
			return nil, fmt.Errorf("load supervised groups: %w", err)
		}
		for _, g := range eduGroups {
			if g != nil {
				groupSet[g.ID] = struct{}{}
			}
		}
	}
	return func(st *userModel.Student) bool {
		if st == nil || st.GroupID == nil {
			return false
		}
		_, ok := groupSet[*st.GroupID]
		return ok
	}, nil
}

// leadMinutes resolves a lead-time setting. A resolution failure is surfaced —
// like the boolean toggle reads, a broken numeric read must fail the request
// rather than silently compute reminders at the wrong time. A non-positive
// configured value is a valid input that normalizes to the registry default.
func (s *service) leadMinutes(ctx context.Context, key string) (int, error) {
	const fallback = 10
	v, err := s.Settings.ResolveInt(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", key, err)
	}
	if v <= 0 {
		return fallback, nil
	}
	return v, nil
}

// overdueThresholdMinutes is how many minutes an activity's start may be
// exceeded before it counts as "not started in time". It reuses the timetable
// setting that already drives the "Überfällig" badge, so both surfaces agree.
// As with leadMinutes, a resolution error is surfaced rather than defaulted.
func (s *service) overdueThresholdMinutes(ctx context.Context) (int, error) {
	const fallback = 5
	v, err := s.Settings.ResolveInt(ctx, configModel.KeyTimetableOverdueThresholdMinutes)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", configModel.KeyTimetableOverdueThresholdMinutes, err)
	}
	if v <= 0 {
		return fallback, nil
	}
	return v, nil
}

// futureBoundary returns boundaryMin when it is strictly after nowMin, else -1
// (the "no candidate" sentinel). Used to feed only genuinely-upcoming state
// flips into the next-change accumulator.
func futureBoundary(boundaryMin, nowMin int) int {
	if boundaryMin > nowMin {
		return boundaryMin
	}
	return -1
}

// minFuture keeps the smaller of two minute-of-day candidates, treating -1 as
// "absent". Both inputs are already restricted to future boundaries by
// futureBoundary, so the result is the soonest upcoming change (-1 if none).
func minFuture(cur, cand int) int {
	if cand < 0 {
		return cur
	}
	if cur < 0 || cand < cur {
		return cand
	}
	return cur
}

// minutesOfDay returns the wall-clock minute of t. TIME columns are stored as a
// fixed-date instant whose Hour/Minute are the wall clock, and timezone.Now()
// is already in Berlin — both sides are wall-clock minutes, so this comparison
// is timezone-safe.
func minutesOfDay(t time.Time) int {
	return t.Hour()*60 + t.Minute()
}

func formatMinutes(m int) string {
	return fmt.Sprintf("%02d:%02d", m/60, m%60)
}

// int64String renders an int64 ID as a *string for JSON, matching
// the repo-wide int64→string API contract.
func int64String(v int64) *string {
	s := strconv.FormatInt(v, 10)
	return &s
}
