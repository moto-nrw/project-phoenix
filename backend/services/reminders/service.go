// Package reminders computes visual-only staff reminders (issue #1457):
// upcoming pickups, overdue pickups, and activity starts. There is no sound and
// no push — the data is derived on request from schedules already in the system
// and rendered on the staff "Erinnerungen" page. All thresholds (which types
// are on, lead-time minutes) are tenant settings resolved at request time.
package reminders

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
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
	Type        string `json:"type"`
	StudentID   *int64 `json:"student_id,omitempty"`
	Title       string `json:"title"`              // student name OR activity title
	Subtitle    string `json:"subtitle,omitempty"` // school class or room — optional
	DueTime     string `json:"due_time"`           // "HH:MM" of the relevant time
	MinutesAway int    `json:"minutes_away"`       // negative when overdue
}

// Result is the computed reminder list plus a convenience count for the badge.
// Enabled reports whether the tenant has switched on at least one reminder
// type — the sidebar uses it to show/hide the "Erinnerungen" entry regardless
// of whether anything is currently due.
type Result struct {
	Reminders []Reminder `json:"reminders"`
	Count     int        `json:"count"`
	Enabled   bool       `json:"enabled"`
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

type studentReader interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModel.Student, error)
}

type personReader interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModel.Person, error)
}

type supervisionReader interface {
	GetStaffActiveSupervisions(ctx context.Context, staffID int64) ([]*activeModel.GroupSupervisor, error)
	GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*activeModel.Group, error)
	ListStudentsPresentInRoom(ctx context.Context, roomID int64) ([]int64, error)
}

// Dependencies wires the readers the service needs. They mirror existing
// services/repositories so no new query construction lives here.
type Dependencies struct {
	Settings    settingsResolver
	Attendance  attendanceReader
	Pickup      pickupReader
	Instance    instanceReader
	Student     studentReader
	Person      personReader
	Supervision supervisionReader
	Logger      *slog.Logger
}

type service struct {
	settings    settingsResolver
	attendance  attendanceReader
	pickup      pickupReader
	instance    instanceReader
	student     studentReader
	person      personReader
	supervision supervisionReader
	logger      *slog.Logger
}

// NewService builds the reminder service.
func NewService(deps Dependencies) Service {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &service{
		settings:    deps.Settings,
		attendance:  deps.Attendance,
		pickup:      deps.Pickup,
		instance:    deps.Instance,
		student:     deps.Student,
		person:      deps.Person,
		supervision: deps.Supervision,
		logger:      logger,
	}
}

func (s *service) Compute(ctx context.Context, scope Scope) (*Result, error) {
	empty := &Result{Reminders: []Reminder{}}
	if s.settings == nil {
		return empty, nil
	}

	pickupUpcoming, _ := s.settings.ResolveBool(ctx, configModel.KeyRemindersPickupUpcomingEnabled)
	pickupOverdue, _ := s.settings.ResolveBool(ctx, configModel.KeyRemindersPickupOverdueEnabled)
	activityStart, _ := s.settings.ResolveBool(ctx, configModel.KeyRemindersActivityStartEnabled)
	activityOverdue, _ := s.settings.ResolveBool(ctx, configModel.KeyRemindersActivityOverdueEnabled)

	// Default-off: nothing enabled means no work and no data exposure.
	if !pickupUpcoming && !pickupOverdue && !activityStart && !activityOverdue {
		return empty, nil
	}

	today := timezone.TodayDate()
	nowMin := minutesOfDay(timezone.Now())

	studentIDs, roomIDs, err := s.resolveScope(ctx, scope, today)
	if err != nil {
		return nil, err
	}

	reminders := make([]Reminder, 0)

	if pickupUpcoming || pickupOverdue {
		lead := s.leadMinutes(ctx, configModel.KeyRemindersPickupUpcomingLeadMinutes)
		pickupReminders, perr := s.pickupReminders(ctx, studentIDs, today, nowMin, lead, pickupUpcoming, pickupOverdue)
		if perr != nil {
			return nil, perr
		}
		reminders = append(reminders, pickupReminders...)
	}

	if activityStart || activityOverdue {
		lead := s.leadMinutes(ctx, configModel.KeyRemindersActivityStartLeadMinutes)
		overdueThreshold := s.overdueThresholdMinutes(ctx)
		activityReminders, aerr := s.activityReminders(ctx, scope, roomIDs, today, nowMin, lead, overdueThreshold, activityStart, activityOverdue)
		if aerr != nil {
			return nil, aerr
		}
		reminders = append(reminders, activityReminders...)
	}

	// Most urgent first (overdue is negative, soonest upcoming next).
	sort.SliceStable(reminders, func(i, j int) bool {
		return reminders[i].MinutesAway < reminders[j].MinutesAway
	})

	return &Result{Reminders: reminders, Count: len(reminders), Enabled: true}, nil
}

// resolveScope returns the present student IDs in scope and, for caregivers,
// the room IDs they supervise (nil for admins, who see all activities).
func (s *service) resolveScope(ctx context.Context, scope Scope, today timezone.Date) ([]int64, []int64, error) {
	if scope.IsAdmin {
		if s.attendance == nil {
			return []int64{}, nil, nil
		}
		ids, err := s.attendance.ListOpenStudentIDsForDate(ctx, today)
		return ids, nil, err
	}

	if s.supervision == nil {
		return []int64{}, []int64{}, nil
	}

	supervisions, err := s.supervision.GetStaffActiveSupervisions(ctx, scope.StaffID)
	if err != nil {
		return nil, nil, err
	}
	groupIDs := make([]int64, 0, len(supervisions))
	for _, sup := range supervisions {
		if sup != nil {
			groupIDs = append(groupIDs, sup.GroupID)
		}
	}
	if len(groupIDs) == 0 {
		return []int64{}, []int64{}, nil
	}

	groups, err := s.supervision.GetActiveGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, nil, err
	}
	roomSet := make(map[int64]struct{}, len(groups))
	for _, g := range groups {
		if g != nil {
			roomSet[g.RoomID] = struct{}{}
		}
	}

	studentSet := make(map[int64]struct{})
	roomIDs := make([]int64, 0, len(roomSet))
	for roomID := range roomSet {
		roomIDs = append(roomIDs, roomID)
		present, perr := s.supervision.ListStudentsPresentInRoom(ctx, roomID)
		if perr != nil {
			return nil, nil, perr
		}
		for _, id := range present {
			studentSet[id] = struct{}{}
		}
	}
	studentIDs := make([]int64, 0, len(studentSet))
	for id := range studentSet {
		studentIDs = append(studentIDs, id)
	}
	return studentIDs, roomIDs, nil
}

func (s *service) pickupReminders(ctx context.Context, studentIDs []int64, today timezone.Date, nowMin, lead int, upcoming, overdue bool) ([]Reminder, error) {
	if len(studentIDs) == 0 || s.pickup == nil {
		return nil, nil
	}
	times, err := s.pickup.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, today)
	if err != nil {
		return nil, err
	}
	names := s.studentNames(ctx, studentIDs)

	out := make([]Reminder, 0)
	for _, id := range studentIDs {
		effective := times[id]
		if effective == nil || effective.PickupTime == nil {
			continue
		}
		pickupMin := minutesOfDay(*effective.PickupTime)
		diff := pickupMin - nowMin
		info := names[id]

		switch {
		case diff < 0 && overdue:
			out = append(out, Reminder{
				Type:        TypePickupOverdue,
				StudentID:   ptrInt64(id),
				Title:       info.name,
				Subtitle:    info.class,
				DueTime:     formatMinutes(pickupMin),
				MinutesAway: diff,
			})
		case diff >= 0 && diff <= lead && upcoming:
			out = append(out, Reminder{
				Type:        TypePickupUpcoming,
				StudentID:   ptrInt64(id),
				Title:       info.name,
				Subtitle:    info.class,
				DueTime:     formatMinutes(pickupMin),
				MinutesAway: diff,
			})
		}
	}
	return out, nil
}

func (s *service) activityReminders(ctx context.Context, scope Scope, roomIDs []int64, today timezone.Date, nowMin, lead, overdueThreshold int, upcoming, overdue bool) ([]Reminder, error) {
	if s.instance == nil {
		return nil, nil
	}
	instances, err := s.instance.FindByTenantAndDate(ctx, today)
	if err != nil {
		return nil, err
	}

	var roomFilter map[int64]struct{}
	if !scope.IsAdmin {
		roomFilter = make(map[int64]struct{}, len(roomIDs))
		for _, id := range roomIDs {
			roomFilter[id] = struct{}{}
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

		switch {
		case upcoming && diff >= 0 && diff <= lead:
			out = append(out, Reminder{
				Type:        TypeActivityStart,
				Title:       inst.Title,
				DueTime:     formatMinutes(startMin),
				MinutesAway: diff,
			})
		// Overdue: planned, started late by at least the threshold, and the
		// slot is not over yet (after end_time a reminder is pointless).
		case overdue && diff < 0 && -diff >= overdueThreshold && nowMin < endMin:
			out = append(out, Reminder{
				Type:        TypeActivityOverdue,
				Title:       inst.Title,
				DueTime:     formatMinutes(startMin),
				MinutesAway: diff,
			})
		}
	}
	return out, nil
}

type studentNameInfo struct {
	name  string
	class string
}

func (s *service) studentNames(ctx context.Context, ids []int64) map[int64]studentNameInfo {
	result := make(map[int64]studentNameInfo, len(ids))
	if s.student == nil {
		return result
	}
	students, err := s.student.FindByIDs(ctx, ids)
	if err != nil {
		s.logger.WarnContext(ctx, "reminders: failed to load student names", "error", err.Error())
		return result
	}
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
	}
	var persons map[int64]*userModel.Person
	if s.person != nil {
		persons, _ = s.person.FindByIDs(ctx, personIDs)
	}
	for id, st := range students {
		info := studentNameInfo{class: st.SchoolClass}
		if persons != nil {
			if p := persons[st.PersonID]; p != nil {
				info.name = p.GetFullName()
			}
		}
		result[id] = info
	}
	return result
}

// leadMinutes resolves a lead-time setting, falling back to the registry
// default when the lookup errors or returns a non-positive value.
func (s *service) leadMinutes(ctx context.Context, key string) int {
	const fallback = 10
	v, err := s.settings.ResolveInt(ctx, key)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

// overdueThresholdMinutes is how many minutes an activity's start may be
// exceeded before it counts as "not started in time". It reuses the timetable
// setting that already drives the "Überfällig" badge, so both surfaces agree.
func (s *service) overdueThresholdMinutes(ctx context.Context) int {
	const fallback = 5
	v, err := s.settings.ResolveInt(ctx, configModel.KeyTimetableOverdueThresholdMinutes)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
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

func ptrInt64(v int64) *int64 {
	return &v
}
