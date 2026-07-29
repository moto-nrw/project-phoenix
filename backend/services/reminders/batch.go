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

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// batchConfig is the tenant-wide configuration of one batch run, resolved once.
// Compute re-reads these per request; here they are shared, which is most of the
// reason the query count stays flat.
type batchConfig struct {
	pickupUpcoming   bool
	pickupOverdue    bool
	activityStart    bool
	activityOverdue  bool
	pickupLead       int
	activityLead     int
	overdueThreshold int
	binaryPresence   bool
	allStaffScope    bool
}

func (c batchConfig) anyPickup() bool   { return c.pickupUpcoming || c.pickupOverdue }
func (c batchConfig) anyActivity() bool { return c.activityStart || c.activityOverdue }
func (c batchConfig) anyEnabled() bool  { return c.anyPickup() || c.anyActivity() }

// batchInputs is everything read once for the whole tenant.
type batchInputs struct {
	today           timezone.Date
	nowMin          int
	attendanceIDs   []int64
	visitsByRoom    map[int64][]int64
	roomsByStaff    map[int64][]int64
	groupsByStaff   map[int64]map[int64]struct{}
	instances       []*scheduleModel.ActivityInstance
	schulhofRoomIDs map[int64]struct{}
	students        map[int64]*userModel.Student
	pickupTimes     map[int64]*scheduleService.EffectivePickupTime
	presenceByStaff map[int64][]int64
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
	pickupsByStaff := make(map[int64]staffPickups, len(recipients))
	dueIDSet := make(map[int64]struct{})

	for _, sc := range recipients {
		if !cfg.anyPickup() {
			pickupsByStaff[sc.StaffID] = staffPickups{nextChange: -1}
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
		pickupsByStaff[sc.StaffID] = staffPickups{dues: dues, nextChange: next}
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
		sp := pickupsByStaff[sc.StaffID]

		pickupPart, dropped := buildPickupReminders(sp.dues, names)
		s.logDroppedPickups(dropped)
		nextChange := sp.nextChange

		var activityPart []Reminder
		if cfg.anyActivity() {
			part, next := buildActivityReminders(in.instances, in.schulhofRoomIDs,
				activityRoomFilter(sc.Scope, in.roomsByStaff[sc.StaffID]), activityWindow{
					nowMin:           in.nowMin,
					lead:             cfg.activityLead,
					overdueThreshold: cfg.overdueThreshold,
					upcoming:         cfg.activityStart,
					overdue:          cfg.activityOverdue,
				})
			activityPart = part
			nextChange = minFuture(nextChange, next)
		}

		results[sc.StaffID] = assembleResult(pickupPart, activityPart, nextChange)
	}

	return results, nil
}

// dedupeBatchScopes drops unusable scopes and collapses repeats.
//
// A non-positive StaffID is dropped rather than computed: it addresses nobody,
// and combined with a bulk lookup that returns no row for it, an empty result
// would be indistinguishable from "no restriction".
func dedupeBatchScopes(scopes []BatchScope) []BatchScope {
	seen := make(map[int64]struct{}, len(scopes))
	out := make([]BatchScope, 0, len(scopes))
	for _, sc := range scopes {
		if sc.StaffID <= 0 {
			continue
		}
		if _, dup := seen[sc.StaffID]; dup {
			continue
		}
		seen[sc.StaffID] = struct{}{}
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
		results[sc.StaffID] = &Result{Reminders: []Reminder{}}
	}
	return results
}

// hasCaregiver reports whether any recipient needs the per-person reads at all.
// A batch of admins only — which is what the notification tick sends today —
// skips all three bulk staff reads.
func hasCaregiver(recipients []BatchScope) bool {
	for _, sc := range recipients {
		if !sc.IsAdmin {
			return true
		}
	}
	return false
}

// loadBatchConfig resolves the tenant settings in the same order and with the
// same error contract as Compute, and only the ones Compute would have read:
// resolving a lead time for a disabled branch would surface an error where
// Compute reports success.
func (s *service) loadBatchConfig(ctx context.Context, recipients []BatchScope) (batchConfig, error) {
	var cfg batchConfig
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
			var scopeVal string
			if scopeVal, err = s.Settings.ResolveString(ctx, configModel.KeyStudentDataScope); err != nil {
				return cfg, fmt.Errorf("resolve %s: %w", configModel.KeyStudentDataScope, err)
			}
			cfg.allStaffScope = scopeVal == configModel.StudentDataScopeAllStaff
		}
	}

	if cfg.anyActivity() {
		if cfg.activityLead, err = s.leadMinutes(ctx, configModel.KeyRemindersActivityStartLeadMinutes); err != nil {
			return cfg, err
		}
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
		today:           timezone.TodayDate(),
		nowMin:          minutesOfDay(timezone.Now()),
		visitsByRoom:    map[int64][]int64{},
		roomsByStaff:    map[int64][]int64{},
		groupsByStaff:   map[int64]map[int64]struct{}{},
		schulhofRoomIDs: map[int64]struct{}{},
		students:        map[int64]*userModel.Student{},
		pickupTimes:     map[int64]*scheduleService.EffectivePickupTime{},
		presenceByStaff: map[int64][]int64{},
	}

	caregivers := hasCaregiver(recipients)

	// Seed an empty entry for every caregiver BEFORE the bulk reads. The readers
	// only return rows for people who have something, and a missing key that is
	// later read as "no restriction" is exactly how a room filter turns into
	// full access.
	for _, sc := range recipients {
		if !sc.IsAdmin {
			in.roomsByStaff[sc.StaffID] = nil
			in.groupsByStaff[sc.StaffID] = map[int64]struct{}{}
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
		if s.BulkVisits != nil {
			byRoom, err := s.BulkVisits.ListOpenVisitStudentIDsByRoom(ctx)
			if err != nil {
				return nil, fmt.Errorf("load open visits by room: %w", err)
			}
			in.visitsByRoom = byRoom
		}
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

	if caregivers && cfg.anyPickup() && !cfg.allStaffScope {
		if s.BulkGroups != nil {
			staffIDs := make([]int64, 0, len(in.groupsByStaff))
			for staffID := range in.groupsByStaff {
				staffIDs = append(staffIDs, staffID)
			}
			pairs, err := s.BulkGroups.ListSupervisedGroupIDsByStaff(ctx, staffIDs, in.today)
			if err != nil {
				return nil, fmt.Errorf("load supervised groups: %w", err)
			}
			for _, pair := range pairs {
				if groups, wanted := in.groupsByStaff[pair.StaffID]; wanted {
					groups[pair.GroupID] = struct{}{}
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
		instances, err := s.Instance.FindByTenantAndDate(ctx, in.today)
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
			schulhof, resolveErr := resolveActivityRooms(rooms, wanted)
			if resolveErr != nil {
				return nil, resolveErr
			}
			in.schulhofRoomIDs = schulhof
		}
	}

	return in, nil
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

// batchReadableStudents applies the gdpr.student_data_scope gate for one
// recipient. The loaded student rows are shared across the batch, but the gate
// is recomputed per person — sharing a gated map would hand one caregiver the
// children another may read.
func (s *service) batchReadableStudents(sc BatchScope, cfg batchConfig, in *batchInputs) (map[int64]*userModel.Student, error) {
	allowAll := sc.IsAdmin || cfg.allStaffScope
	groups := in.groupsByStaff[sc.StaffID]

	readable := make(map[int64]*userModel.Student)
	for _, id := range in.presenceByStaff[sc.StaffID] {
		st := in.students[id]
		if st == nil {
			continue
		}
		if !allowAll {
			if st.GroupID == nil {
				continue
			}
			if _, ok := groups[*st.GroupID]; !ok {
				continue
			}
		}
		readable[id] = st
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
