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
package application

import (
	"context"
	"strconv"

	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
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
	runningOver      bool
	assignedStart    bool
	pickupLead       int
	activityLead     int
	overdueThreshold int
	runningOverGrace int
	binaryPresence   bool
}

func (c batchConfig) anyPickup() bool { return c.pickupUpcoming || c.pickupOverdue }
func (c batchConfig) anyActivity() bool {
	return c.activityStart || c.activityOverdue || c.runningOver || c.assignedStart
}
func (c batchConfig) anyEnabled() bool { return c.anyPickup() || c.anyActivity() }

// batchInputs is everything read once for the whole tenant.
type batchInputs struct {
	today           string
	nowMin          int
	attendanceIDs   []int64
	visitsByRoom    map[int64][]int64
	roomsByStaff    map[int64][]int64
	instances       []*ports.ActivityInstance
	students        map[int64]*ports.Student
	pickupTimes     map[int64]*ports.EffectivePickupTime
	presenceByStaff map[int64][]int64
	// assignedInstances holds, per staff member, the activity instances they
	// are planned on and not absent from.
	assignedInstances map[int64]map[int64]struct{}
	// runningOver holds the rooms, room presence and pickup times of every
	// block that runs over anywhere in the school, read once; each recipient
	// then sees only the blocks in their own scope.
	runningOver runningOverFacts
}

func (s *service) ComputeBatch(ctx context.Context, scopes []reminder.BatchScope) (map[int64]*reminder.Result, error) {
	recipients := dedupeBatchScopes(scopes)
	results := make(map[int64]*reminder.Result, len(recipients))
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

	pickupsByRecipient, dueIDSet, err := s.batchDuePickups(ctx, recipients, cfg, in)
	if err != nil {
		return nil, err
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

	results = s.renderBatchResults(recipients, pickupsByRecipient, names, cfg, in)
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
func dedupeBatchScopes(scopes []reminder.BatchScope) []reminder.BatchScope {
	positions := make(map[int64]int, len(scopes))
	out := make([]reminder.BatchScope, 0, len(scopes))
	for _, sc := range scopes {
		key := resultKey(sc)
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
func disabledResults(recipients []reminder.BatchScope) map[int64]*reminder.Result {
	results := make(map[int64]*reminder.Result, len(recipients))
	for _, sc := range recipients {
		results[resultKey(sc)] = &reminder.Result{Reminders: []reminder.Reminder{}}
	}
	return results
}

// hasCaregiver reports whether any recipient needs the supervision and group
// reads. Planned assignments are loaded for every recipient, including admins.
func hasCaregiver(recipients []reminder.BatchScope) bool {
	for _, sc := range recipients {
		if !sc.IsAdmin {
			return true
		}
	}
	return false
}

func hasAssignedActivityStart(recipients []reminder.BatchScope) bool {
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
func (s *service) loadBatchConfig(ctx context.Context, recipients []reminder.BatchScope) (batchConfig, error) {
	cfg := batchConfig{assignedStart: hasAssignedActivityStart(recipients)}
	var err error

	if cfg.pickupUpcoming, err = s.Settings.PickupUpcomingEnabled(ctx); err != nil {
		return cfg, err
	}
	if cfg.pickupOverdue, err = s.Settings.PickupOverdueEnabled(ctx); err != nil {
		return cfg, err
	}
	gates, err := s.activityGates(ctx)
	if err != nil {
		return cfg, err
	}
	cfg.activityStart, cfg.activityOverdue, cfg.runningOver = gates.start, gates.overdue, gates.runningOver
	if !cfg.anyEnabled() {
		return cfg, nil
	}

	caregivers := hasCaregiver(recipients)

	if err := s.loadBatchPickupConfig(ctx, &cfg, caregivers); err != nil {
		return cfg, err
	}
	if cfg.activityStart || cfg.assignedStart {
		if cfg.activityLead, err = s.leadMinutes(ctx, s.Settings.ActivityLeadMinutes); err != nil {
			return cfg, err
		}
	}
	if err := s.loadBatchActivityThresholds(ctx, &cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// loadBatchInputs performs every tenant-wide read exactly once, then derives the
// per-person presence sets in memory.
func (s *service) loadBatchInputs(ctx context.Context, cfg batchConfig, recipients []reminder.BatchScope) (*batchInputs, error) {
	today, nowMin := s.Clock()
	in := &batchInputs{
		today:             today,
		nowMin:            nowMin,
		visitsByRoom:      map[int64][]int64{},
		roomsByStaff:      map[int64][]int64{},
		students:          map[int64]*ports.Student{},
		pickupTimes:       map[int64]*ports.EffectivePickupTime{},
		presenceByStaff:   map[int64][]int64{},
		assignedInstances: map[int64]map[int64]struct{}{},
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

	caregivers := hasCaregiver(recipients)
	if err := s.loadBatchPresence(ctx, cfg, recipients, caregivers, in); err != nil {
		return nil, err
	}
	if err := s.loadBatchSupervision(ctx, cfg, caregivers, in); err != nil {
		return nil, err
	}
	if cfg.anyPickup() {
		if err := s.loadBatchPickups(ctx, cfg, recipients, in); err != nil {
			return nil, err
		}
	}
	if cfg.anyActivity() && s.Instance != nil {
		if err := s.loadBatchActivities(ctx, cfg, recipients, in); err != nil {
			return nil, err
		}
	}
	return in, nil
}

func assignedActivityFilter(instanceIDs map[int64]struct{}) func(*ports.ActivityInstance) bool {
	return func(inst *ports.ActivityInstance) bool {
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
) func(*ports.ActivityInstance) bool {
	assigned := assignedActivityFilter(instanceIDs)
	return func(inst *ports.ActivityInstance) bool {
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
func (s *service) batchPresence(sc reminder.BatchScope, cfg batchConfig, in *batchInputs) []int64 {
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
func (s *service) batchReadableStudents(sc reminder.BatchScope, _ batchConfig, in *batchInputs) (map[int64]*ports.Student, error) {
	readable := make(map[int64]*ports.Student)
	for _, id := range in.presenceByStaff[sc.StaffID] {
		if st := in.students[id]; st != nil {
			readable[id] = st
		}
	}
	return readable, nil
}

func hasAdmin(recipients []reminder.BatchScope) bool {
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

type staffPickups struct {
	dues       []duePickup
	nextChange int
}

func (s *service) batchDuePickups(ctx context.Context, recipients []reminder.BatchScope, cfg batchConfig, in *batchInputs) (map[int64]staffPickups, map[int64]struct{}, error) {
	// Per-staff pass one: which pickups are due, and the soonest boundary. Both
	// are per person by construction — a shared next-change would leak the
	// earliest pickup minute in the school to everyone.
	pickupsByRecipient := make(map[int64]staffPickups, len(recipients))
	dueIDSet := make(map[int64]struct{})

	for _, sc := range recipients {
		resultKey := resultKey(sc)
		if !cfg.anyPickup() {
			pickupsByRecipient[resultKey] = staffPickups{nextChange: -1}
			continue
		}
		readable, rerr := s.batchReadableStudents(sc, cfg, in)
		if rerr != nil {
			return nil, nil, rerr
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

	return pickupsByRecipient, dueIDSet, nil
}

func batchActivityResult(sc reminder.BatchScope, cfg batchConfig, in *batchInputs, nextChange int) ([]reminder.Reminder, int) {
	var activityPart []reminder.Reminder
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
		if cfg.runningOver {
			due, runningNext := dueRunningOver(in.instances, activityScopeFilter(roomFilter, nil),
				runningOverWindow{nowMin: in.nowMin, grace: cfg.runningOverGrace})
			part = append(part, buildRunningOverReminders(due, in.nowMin, in.runningOver)...)
			next = minFuture(next, runningNext)
		}
		activityPart = part
		nextChange = minFuture(nextChange, next)
	}

	return activityPart, nextChange
}

// loadBatchActivityThresholds resolves the minute settings of the switched-on
// activity types that are evaluated after their start.
func (s *service) loadBatchActivityThresholds(ctx context.Context, cfg *batchConfig) error {
	var err error
	if cfg.activityOverdue {
		if cfg.overdueThreshold, err = s.overdueThresholdMinutes(ctx); err != nil {
			return err
		}
	}
	if cfg.runningOver {
		if cfg.runningOverGrace, err = s.runningOverMinutes(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) loadBatchPickupConfig(ctx context.Context, cfg *batchConfig, caregivers bool) error {
	var err error
	if cfg.anyPickup() {
		if cfg.pickupLead, err = s.leadMinutes(ctx, s.Settings.PickupLeadMinutes); err != nil {
			return err
		}
		if caregivers {
			if cfg.binaryPresence, err = s.binaryMode(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *service) renderBatchResults(recipients []reminder.BatchScope, pickupsByRecipient map[int64]staffPickups, names map[int64]studentNameInfo, cfg batchConfig, in *batchInputs) map[int64]*reminder.Result {
	results := make(map[int64]*reminder.Result, len(recipients))
	// Per-staff pass two: render.
	for _, sc := range recipients {
		sp := pickupsByRecipient[resultKey(sc)]

		pickupPart, dropped := buildPickupReminders(sp.dues, names)
		s.logDroppedPickups(dropped)
		nextChange := sp.nextChange

		activityPart, nextChange := batchActivityResult(sc, cfg, in, nextChange)
		result := assembleResult(pickupPart, activityPart, nextChange)
		result.AssignedActivityInstanceIDs = assignedIDStrings(in.assignedInstances[sc.StaffID])
		results[resultKey(sc)] = result
	}

	return results
}
