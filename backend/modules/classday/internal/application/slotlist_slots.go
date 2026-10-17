package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

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
		if !selectableSlot(inst, params.ListKind) {
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
		if !slotSelected(selected, inst.ID) {
			continue
		}
		process = append(process, sc)
	}
	return process, deferredInstances, nil
}

// selectableSlot excludes cancelled occurrences before matching the list kind.
func selectableSlot(inst *slotInstance, listKind classday.ListKind) bool {
	return inst.Status != ports.InstanceStatusCancelled && instanceMatchesListKind(inst, listKind)
}

// slotSelected distinguishes the default all-slots selection from an explicit
// empty selection, without restricting the published slot options.
func slotSelected(selected map[int64]struct{}, instanceID int64) bool {
	if selected == nil {
		return true
	}
	_, ok := selected[instanceID]
	return ok
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
func (s *service) loadSlotPresence(ctx context.Context, process []slotContext, date calendar.Date) (
	map[int64][]ports.InstanceStudent,
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
	rosterByInstance := make(map[int64][]ports.InstanceStudent, len(process))
	careDay := map[int64]ports.CareDay{}
	// presentByGroup maps an active-group ID to the students seen present in it.
	// A slot only has visit evidence once it carries an ActiveGroupID, so this is
	// keyed by group, not instance; instances without one (never started — e.g.
	// most Plan requests) contribute nothing and cost no query.
	presentByGroup := map[int64]map[int64]bool{}
	if len(process) == 0 {
		return rosterByInstance, careDay, presentByGroup, nil
	}
	instanceIDs, activeGroupIDs := slotPresenceIDs(process)
	rosterRows, err := s.loadRoster(ctx, instanceIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	studentIDs := indexSlotRoster(rosterRows, rosterByInstance)
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

// slotPresenceIDs builds the two batches without dropping repeated group IDs.
func slotPresenceIDs(process []slotContext) ([]int64, []int64) {
	instanceIDs := make([]int64, 0, len(process))
	activeGroupIDs := make([]int64, 0, len(process))
	for _, p := range process {
		instanceIDs = append(instanceIDs, p.inst.ID)
		if p.inst.ActiveGroupID != nil {
			activeGroupIDs = append(activeGroupIDs, *p.inst.ActiveGroupID)
		}
	}
	return instanceIDs, activeGroupIDs
}

// indexSlotRoster groups roster rows and returns the distinct care-day subjects.
func indexSlotRoster(rosterRows []ports.InstanceStudent, rosterByInstance map[int64][]ports.InstanceStudent) []int64 {
	studentIDSet := map[int64]struct{}{}
	for _, row := range rosterRows {
		rosterByInstance[row.InstanceID] = append(rosterByInstance[row.InstanceID], row)
		studentIDSet[row.StudentID] = struct{}{}
	}
	studentIDs := make([]int64, 0, len(studentIDSet))
	for id := range studentIDSet {
		studentIDs = append(studentIDs, id)
	}
	return studentIDs
}

// mergeSlotInstance merges one instance's roster (Plan) with its visit-derived
// presence (Ist) into display entries and records the export "Enthalten"
// accounting for void-plan slots.
func (s *service) mergeSlotInstance(
	p slotContext,
	rosterByInstance map[int64][]ports.InstanceStudent,
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
	completed := inst.Status == ports.InstanceStatusCompleted

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
	planned []ports.InstanceStudent,
	presentSet map[int64]bool,
	completed bool,
	careDay map[int64]ports.CareDay,
	seenPlanned map[int64]struct{},
) []mergedEntry {
	entries := []mergedEntry{}
	for _, row := range planned {
		present := presentSet[row.StudentID] || row.Status == ports.AttendancePresent
		// A manual attendance correction is a human decision that outranks stale
		// visit evidence. UpdateAttendanceFields stamps ManualStatusAt whenever
		// staff set the row's status by hand, so when an erroneous scan created a
		// visit (row still in presentSet) and staff then corrected the roster to
		// absent or expected, the visit must NOT force Present=true: trust the
		// corrected row status alone. Without this the Ist/Abgleich lists and their
		// counters would report the child "Anwesend" against the explicit human
		// correction (#1565 review pass 12).
		if row.ManualStatusAt != nil {
			present = row.Status == ports.AttendancePresent
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
	planned []ports.InstanceStudent,
	careDay map[int64]ports.CareDay,
	seenPlanned map[int64]struct{},
) ([]mergedEntry, bool) {
	entries := []mergedEntry{}
	retained := false
	for _, row := range planned {
		switch row.Status {
		case ports.AttendancePresent:
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
				PlannedStatus:    ports.AttendanceAbsent,
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
	row ports.InstanceStudent,
	present bool,
	verdict, rawCareDay ports.CareDay,
) ([]mergedEntry, bool) {
	inst := p.inst
	completed := inst.Status == ports.InstanceStatusCompleted
	switch {
	case row.IsUnplanned:
		// #1913: the row was created by an observed walk-in visit, not by
		// planning. It is durable presence evidence, never a plan entry —
		// otherwise the Abgleich would relabel the walk-in as "geplant & anwesend".
		return presentOnlyEntry(p, row.StudentID, present), true
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
		return presentOnlyEntry(p, row.StudentID, present), true
	case row.ManualStatusAt == nil && rawCareDay == ports.CareDayCancelled:
		return cancelledPlannedEntry(p, row, present)
	default:
		return classifyRosterExpectation(p, row, present, verdict)
	}
}

// classifyRosterExpectation applies the roster's frozen expectation after the
// unplanned and live care-day cases have been handled.
func classifyRosterExpectation(p slotContext, row ports.InstanceStudent, present bool, verdict ports.CareDay) ([]mergedEntry, bool) {
	switch {
	case row.Status == ports.AttendanceExpected && !verdict.Expected():
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
		return presentOnlyEntry(p, row.StudentID, present), true
	default:
		return []mergedEntry{{
			StudentID:        row.StudentID,
			InstanceID:       p.inst.ID,
			SlotLabel:        p.slotLabel,
			RoomName:         p.roomName,
			Planned:          true,
			Present:          present,
			PlannedStatus:    row.Status,
			PlannedSubstatus: row.Substatus,
		}}, true
	}
}

// presentOnlyEntry preserves observed attendance without inventing a plan entry.
func presentOnlyEntry(p slotContext, studentID int64, present bool) []mergedEntry {
	if !present {
		return nil
	}
	return []mergedEntry{{
		StudentID:  studentID,
		InstanceID: p.inst.ID,
		SlotLabel:  p.slotLabel,
		RoomName:   p.roomName,
		Present:    true,
	}}
}

// cancelledPlannedEntry keeps a registered sign-off unless attendance supersedes it.
func cancelledPlannedEntry(p slotContext, row ports.InstanceStudent, present bool) ([]mergedEntry, bool) {
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
		return presentOnlyEntry(p, row.StudentID, present), true
	}
	cancelledSubstatus := string(ports.CareDayCancelled)
	return []mergedEntry{{
		StudentID:        row.StudentID,
		InstanceID:       p.inst.ID,
		SlotLabel:        p.slotLabel,
		RoomName:         p.roomName,
		Planned:          true,
		Present:          false,
		PlannedStatus:    ports.AttendanceAbsent,
		PlannedSubstatus: &cancelledSubstatus,
	}}, true
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
func (s *service) beforeEffectiveArrival(date calendar.Date, arrival *time.Time) bool {
	if arrival == nil {
		return false
	}
	at := *arrival
	start := time.Date(
		date.Year(), date.Month(), date.Day(),
		at.Hour(), at.Minute(), at.Second(), at.Nanosecond(),
		calendar.Berlin,
	)
	return s.currentTime().Before(start)
}

func instanceTimeRange(inst *slotInstance) (time.Time, time.Time) {
	start := time.Date(
		inst.Date.Year(), inst.Date.Month(), inst.Date.Day(),
		inst.StartTime.Hour(), inst.StartTime.Minute(), inst.StartTime.Second(), inst.StartTime.Nanosecond(),
		calendar.Berlin,
	)
	end := time.Date(
		inst.Date.Year(), inst.Date.Month(), inst.Date.Day(),
		inst.EndTime.Hour(), inst.EndTime.Minute(), inst.EndTime.Second(), inst.EndTime.Nanosecond(),
		calendar.Berlin,
	)
	return start, end
}
