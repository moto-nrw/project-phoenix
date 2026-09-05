// Batch computation (epic "persönliche Benachrichtigungen"): ComputeBatch
// answers the same question as Compute for many staff members at once, with a
// number of queries that does not grow with the number of staff.
//
// Two properties make it worth a separate entry point rather than a loop over
// Compute. First, Compute's read gate resolves the caller from JWT claims, so a
// background job cannot use it at all. Second, of the ~28 queries Compute issues
// for one caregiver, only three facts are genuinely per-person; everything else
// is identical for the whole tenant.
//
// The actual reminder rules are NOT reimplemented here. Both entry points call
// the same pure cores in service.go (duePickups, buildPickupReminders,
// resolveActivityRooms, buildActivityReminders, assembleResult), and an
// equivalence test asserts that ComputeBatch and Compute agree scope by scope.
// If you are about to add a rule here, it belongs in a pure core instead.
package reminders

import (
	"context"
	"fmt"
	"strconv"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// batchConfig combines the tenant-wide settings resolved once per batch with
// whether any recipient requested the ungated personal-assignment type.
// Compute re-reads tenant settings per request; sharing them here is most of
// the reason the query count stays flat.
type batchConfig struct {
	pickupUpcoming   bool
	pickupOverdue    bool
	activityStart    bool
	activityOverdue  bool
	assignedStart    bool
	pickupLead       int
	activityLead     int
	overdueThreshold int
	binaryPresence   bool
}

func (c batchConfig) anyPickup() bool { return c.pickupUpcoming || c.pickupOverdue }
func (c batchConfig) anyActivity() bool {
	return c.activityStart || c.activityOverdue || c.assignedStart
}
func (c batchConfig) anyEnabled() bool { return c.anyPickup() || c.anyActivity() }

// batchInputs is everything read once for the whole tenant.
type batchInputs struct {
	today           timezone.Date
	nowMin          int
	attendanceIDs   []int64
	visitsByRoom    map[int64][]int64
	roomsByStaff    map[int64][]int64
	instances       []*scheduleModel.ActivityInstance
	students        map[int64]*userModel.Student
	pickupTimes     map[int64]*scheduleService.EffectivePickupTime
	presenceByStaff map[int64][]int64
	// assignedInstances holds, per staff member, the activity instances they
	// are planned on and not absent from.
	assignedInstances map[int64]map[int64]struct{}
}

func (s *service) ComputeBatch(ctx context.Context, scopes []BatchScope) (map[int64]*Result, error) {
	recipients := dedupeBatchScopes(scopes)
	results := make(map[int64]*Result, len(recipients))
	if len(recipients) == 0 {
		return results, nil
	}

	// Mirrors Compute's nil-settings guard: an empty, disabled result rather
	// than a failure, so a partially wired service degrades the same way.
	if s.Settings == nil {
		return disabledResults(recipients), nil
	}

	var err error
	ctx, err = s.withSettingsSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	cfg, err := s.loadBatchConfig(ctx, recipients)
	if err != nil {
		return nil, err
	}
	if !cfg.anyEnabled() {
		return disabledResults(recipients), nil
	}

	in, err := s.loadBatchInputs(ctx, cfg, recipients)
	if err != nil {
		return nil, err
	}

	// Per-staff pass one: which pickups are due, and the soonest boundary. Both
	// are per person by construction — a shared next-change would leak the
	// earliest pickup minute in the school to everyone.
	type staffPickups struct {
		dues       []duePickup
		nextChange int
	}
	pickupsByRecipient := make(map[int64]staffPickups, len(recipients))
	dueIDSet := make(map[int64]struct{})

	for _, sc := range recipients {
		resultKey := sc.resultKey()
		if !cfg.anyPickup() {
			pickupsByRecipient[resultKey] = staffPickups{nextChange: -1}
			continue
		}
		readable, rerr := s.batchReadableStudents(sc, cfg, in)
		if rerr != nil {
			return nil, rerr
		}
		dues, next := duePickups(in.presenceByStaff[sc.StaffID], in.pickupTimes, readable, pickupWindow{
			nowMin:   in.nowMin,
			lead:     cfg.pickupLead,
			upcoming: cfg.pickupUpcoming,
			overdue:  cfg.pickupOverdue,
		})
		pickupsByRecipient[resultKey] = staffPickups{dues: dues, nextChange: next}
		for _, d := range dues {
			dueIDSet[d.id] = struct{}{}
		}
	}

	// One name hydration for the union of everyone's due students. Deliberately
	// not derived from the admin set: in detailed mode a caregiver's presence
	// comes from active.visits while an admin's comes from active.attendance,
	// and those two are not in a subset relation, so an admin-derived set would
	// silently drop a child that is in the caregiver's own room.
	names, err := s.hydrateNames(ctx, in.students, sortedIDs(dueIDSet))
	if err != nil {
		return nil, err
	}

	// Per-staff pass two: render.
	for _, sc := range recipients {
		sp := pickupsByRecipient[sc.resultKey()]

		pickupPart, dropped := buildPickupReminders(sp.dues, names)
		s.logDroppedPickups(dropped)
		nextChange := sp.nextChange

		var activityPart []Reminder
		if cfg.anyActivity() {
			roomFilter := activityRoomFilter(sc.Scope, in.roomsByStaff[sc.StaffID])
			part, next := buildActivityReminders(in.instances,
				activityScopeFilter(roomFilter, nil), activityWindow{
					nowMin:           in.nowMin,
					lead:             cfg.activityLead,
					overdueThreshold: cfg.overdueThreshold,
					upcoming:         cfg.activityStart,
					overdue:          cfg.activityOverdue,
				})
			if sc.IncludeAssignedActivityStart {
				assignedPart, assignedNext := buildActivityReminders(
					in.instances,
					assignedUpcomingFilter(
						in.assignedInstances[sc.StaffID],
						roomFilter,
						cfg.activityStart,
					),
					activityWindow{
						nowMin:   in.nowMin,
						lead:     cfg.activityLead,
						upcoming: true,
					},
				)
				part = append(part, assignedPart...)
				next = minFuture(next, assignedNext)
			}
			activityPart = part
			nextChange = minFuture(nextChange, next)
		}

		result := assembleResult(pickupPart, activityPart, nextChange)
		result.AssignedActivityInstanceIDs = assignedIDStrings(in.assignedInstances[sc.StaffID])
		results[sc.resultKey()] = result
	}

	return results, nil
}

// assignedIDStrings renders an instance-ID set in the same string form the
// reminders themselves carry, so a consumer can match the two without knowing
// how IDs are formatted. Nil for a person with no assignment, which reads as
// "nothing of mine" rather than as an empty restriction.
func assignedIDStrings(instanceIDs map[int64]struct{}) map[string]struct{} {
	if len(instanceIDs) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(instanceIDs))
	for id := range instanceIDs {
		out[strconv.FormatInt(id, 10)] = struct{}{}
	}
	return out
}

// dedupeBatchScopes drops unusable scopes and collapses repeats.
//
// A staff-less admin is valid when the caller supplies an explicit result key:
// admin visibility does not depend on a staff row. Non-admins still require a
// positive StaffID so an absent bulk row can never widen their read scope.
func dedupeBatchScopes(scopes []BatchScope) []BatchScope {
	positions := make(map[int64]int, len(scopes))
	out := make([]BatchScope, 0, len(scopes))
	for _, sc := range scopes {
		key := sc.resultKey()
		if key == 0 || sc.StaffID < 0 || (sc.StaffID == 0 && !sc.IsAdmin) {
			continue
		}
		if pos, dup := positions[key]; dup {
			out[pos].IncludeAssignedActivityStart = out[pos].IncludeAssignedActivityStart || sc.IncludeAssignedActivityStart
			continue
		}
		positions[key] = len(out)
		out = append(out, sc)
	}
	return out
}

// disabledResults gives every recipient its own empty, disabled result. Each
// gets a distinct Result and a distinct slice: sharing one would couple two
// people's lists.
func disabledResults(recipients []BatchScope) map[int64]*Result {
	results := make(map[int64]*Result, len(recipients))
	for _, sc := range recipients {
		results[sc.resultKey()] = &Result{Reminders: []Reminder{}}
	}
	return results
}

// hasCaregiver reports whether any recipient needs the supervision and group
// reads. Planned assignments are loaded for every recipient, including admins.
func hasCaregiver(recipients []BatchScope) bool {
	for _, sc := range recipients {
		if !sc.IsAdmin {
			return true
		}
	}
	return false
}

func hasAssignedActivityStart(recipients []BatchScope) bool {
	for _, sc := range recipients {
		if sc.IncludeAssignedActivityStart {
			return true
		}
	}
	return false
}

// loadBatchConfig resolves the tenant settings in the same order and with the
// same error contract as Compute. The personal assignment type additionally
// needs the activity lead time even when the room-scoped start gate is off.
func (s *service) loadBatchConfig(ctx context.Context, recipients []BatchScope) (batchConfig, error) {
	cfg := batchConfig{assignedStart: hasAssignedActivityStart(recipients)}
	var err error

	if cfg.pickupUpcoming, err = s.resolveToggle(ctx, configModel.KeyRemindersPickupUpcomingEnabled); err != nil {
		return cfg, err
	}
	if cfg.pickupOverdue, err = s.resolveToggle(ctx, configModel.KeyRemindersPickupOverdueEnabled); err != nil {
		return cfg, err
	}
	if cfg.activityStart, err = s.resolveToggle(ctx, configModel.KeyRemindersActivityStartEnabled); err != nil {
		return cfg, err
	}
	if cfg.activityOverdue, err = s.resolveToggle(ctx, configModel.KeyRemindersActivityOverdueEnabled); err != nil {
		return cfg, err
	}
	if !cfg.anyEnabled() {
		return cfg, nil
	}

	caregivers := hasCaregiver(recipients)

	if cfg.anyPickup() {
		if cfg.pickupLead, err = s.leadMinutes(ctx, configModel.KeyRemindersPickupUpcomingLeadMinutes); err != nil {
			return cfg, err
		}
		if caregivers {
			if cfg.binaryPresence, err = s.binaryMode(ctx); err != nil {
				return cfg, err
			}
		}
	}

	if cfg.activityStart || cfg.assignedStart {
		if cfg.activityLead, err = s.leadMinutes(ctx, configModel.KeyRemindersActivityStartLeadMinutes); err != nil {
			return cfg, err
		}
	}
	if cfg.activityOverdue {
		if cfg.overdueThreshold, err = s.overdueThresholdMinutes(ctx); err != nil {
			return cfg, err
		}
	}

	return cfg, nil
}

func (s *service) resolveToggle(ctx context.Context, key string) (bool, error) {
	v, err := s.Settings.ResolveBool(ctx, key)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", key, err)
	}
	return v, nil
}

// loadBatchInputs performs every tenant-wide read exactly once, then derives the
// per-person presence sets in memory.
func (s *service) loadBatchInputs(ctx context.Context, cfg batchConfig, recipients []BatchScope) (*batchInputs, error) {
	in := &batchInputs{
		today:             timezone.TodayDate(),
		nowMin:            minutesOfDay(timezone.Now()),
		visitsByRoom:      map[int64][]int64{},
		roomsByStaff:      map[int64][]int64{},
		students:          map[int64]*userModel.Student{},
		pickupTimes:       map[int64]*scheduleService.EffectivePickupTime{},
		presenceByStaff:   map[int64][]int64{},
		assignedInstances: map[int64]map[int64]struct{}{},
	}

	caregivers := hasCaregiver(recipients)
	recipientStaffIDs := make(map[int64]struct{}, len(recipients))
	for _, sc := range recipients {
		if sc.StaffID > 0 {
			recipientStaffIDs[sc.StaffID] = struct{}{}
		}
	}

	// Seed an empty entry for every caregiver BEFORE the bulk reads. The readers
	// only return rows for people who have something, and a missing key that is
	// later read as "no restriction" is exactly how a room filter turns into
	// full access.
	for _, sc := range recipients {
		if !sc.IsAdmin {
			in.roomsByStaff[sc.StaffID] = nil
		}
	}

	needsAttendance := cfg.anyPickup() && (!caregivers || cfg.binaryPresence || hasAdmin(recipients))
	if needsAttendance {
		ids, err := s.presentStudentIDsFromAttendance(ctx, in.today)
		if err != nil {
			return nil, err
		}
		in.attendanceIDs = ids
	}

	if caregivers && cfg.anyPickup() && !cfg.binaryPresence {
		byRoom, err := s.Visits.ListOpenVisitStudentIDsByRoom(ctx)
		if err != nil {
			return nil, fmt.Errorf("load open visits by room: %w", err)
		}
		in.visitsByRoom = byRoom
	}

	if caregivers && (cfg.anyActivity() || (cfg.anyPickup() && !cfg.binaryPresence)) {
		if s.BulkSupervision != nil {
			rows, err := s.BulkSupervision.ListActiveSupervisedRooms(ctx)
			if err != nil {
				return nil, fmt.Errorf("load supervised rooms: %w", err)
			}
			for _, row := range rows {
				if _, wanted := in.roomsByStaff[row.StaffID]; wanted {
					in.roomsByStaff[row.StaffID] = append(in.roomsByStaff[row.StaffID], row.RoomID)
				}
			}
		}
	}

	// Presence per person, reproducing pickupScopeStudentIDs branch for branch.
	if cfg.anyPickup() {
		for _, sc := range recipients {
			in.presenceByStaff[sc.StaffID] = s.batchPresence(sc, cfg, in)
		}
	}

	if cfg.anyPickup() {
		union := unionOf(in.presenceByStaff)
		if len(union) > 0 {
			if s.Student != nil {
				students, err := s.Student.FindReadScopeByIDs(ctx, union)
				if err != nil {
					return nil, fmt.Errorf("load students: %w", err)
				}
				in.students = students
			}
			if s.Pickup != nil {
				times, err := s.Pickup.GetBulkEffectivePickupTimesForDate(ctx, union, in.today)
				if err != nil {
					return nil, err
				}
				in.pickupTimes = times
			}
		}
	}

	if cfg.anyActivity() && s.Instance != nil {
		instances, err := s.Instance.FindByTenantAndDate(ctx, scheduleModel.Date(in.today))
		if err != nil {
			return nil, err
		}
		in.instances = instances

		if cfg.activityOverdue {
			if s.Room == nil {
				return nil, errActivityRoomReaderMissing
			}
			wanted := plannedInstanceRoomIDs(instances)
			rooms, roomErr := s.Room.FindByIDs(ctx, sortedIDs(wanted))
			if roomErr != nil {
				return nil, fmt.Errorf("load activity reminder rooms: %w", roomErr)
			}
			if resolveErr := resolveActivityRooms(rooms, wanted); resolveErr != nil {
				return nil, resolveErr
			}
		}

		// Planned assignments, one query for the whole day. Absent rows are
		// skipped: someone who called in sick must not be pinged about the
		// slot they were relieved of. Substitutes are kept — they are the ones
		// who have to show up.
		if s.BulkInstanceStaff != nil {
			instanceIDs := make([]int64, 0, len(instances))
			for _, inst := range instances {
				if inst != nil && inst.Status == scheduleModel.InstanceStatusPlanned {
					instanceIDs = append(instanceIDs, inst.ID)
				}
			}
			rows, staffErr := s.BulkInstanceStaff.FindByInstanceIDs(ctx, instanceIDs)
			if staffErr != nil {
				return nil, fmt.Errorf("load planned activity staff: %w", staffErr)
			}
			for _, row := range rows {
				if row == nil || row.IsAbsent {
					continue
				}
				if _, wanted := recipientStaffIDs[row.StaffID]; !wanted {
					continue
				}
				if in.assignedInstances[row.StaffID] == nil {
					in.assignedInstances[row.StaffID] = map[int64]struct{}{}
				}
				in.assignedInstances[row.StaffID][row.InstanceID] = struct{}{}
			}
		}
	}

	return in, nil
}

func assignedActivityFilter(instanceIDs map[int64]struct{}) func(*scheduleModel.ActivityInstance) bool {
	return func(inst *scheduleModel.ActivityInstance) bool {
		_, assigned := instanceIDs[inst.ID]
		return assigned
	}
}

// assignedUpcomingFilter adds only personal starts the room-scoped pass did
// not already include. Assignments never widen overdue reminders.
func assignedUpcomingFilter(
	instanceIDs map[int64]struct{},
	roomFilter map[int64]struct{},
	roomUpcomingEnabled bool,
) func(*scheduleModel.ActivityInstance) bool {
	assigned := assignedActivityFilter(instanceIDs)
	return func(inst *scheduleModel.ActivityInstance) bool {
		if !assigned(inst) {
			return false
		}
		if !roomUpcomingEnabled {
			return true
		}
		if roomFilter == nil {
			return false
		}
		_, alreadyIncluded := roomFilter[inst.RoomID]
		return !alreadyIncluded
	}
}

// batchPresence reproduces pickupScopeStudentIDs for one recipient, from
// already-loaded data.
func (s *service) batchPresence(sc BatchScope, cfg batchConfig, in *batchInputs) []int64 {
	if sc.IsAdmin || cfg.binaryPresence {
		return in.attendanceIDs
	}
	present := make(map[int64]struct{})
	for _, roomID := range in.roomsByStaff[sc.StaffID] {
		for _, studentID := range in.visitsByRoom[roomID] {
			present[studentID] = struct{}{}
		}
	}
	return sortedIDs(present)
}

// batchReadableStudents resolves the loaded student rows for one recipient.
// Every recipient is staff, so since #2329 the whole presence set is readable —
// per-group gating is gone.
func (s *service) batchReadableStudents(sc BatchScope, _ batchConfig, in *batchInputs) (map[int64]*userModel.Student, error) {
	readable := make(map[int64]*userModel.Student)
	for _, id := range in.presenceByStaff[sc.StaffID] {
		if st := in.students[id]; st != nil {
			readable[id] = st
		}
	}
	return readable, nil
}

func hasAdmin(recipients []BatchScope) bool {
	for _, sc := range recipients {
		if sc.IsAdmin {
			return true
		}
	}
	return false
}

// unionOf collapses the per-person presence sets into one ascending ID list, so
// the shared student and pickup-time reads happen once.
func unionOf(byStaff map[int64][]int64) []int64 {
	set := make(map[int64]struct{})
	for _, ids := range byStaff {
		for _, id := range ids {
			set[id] = struct{}{}
		}
	}
	return sortedIDs(set)
}
