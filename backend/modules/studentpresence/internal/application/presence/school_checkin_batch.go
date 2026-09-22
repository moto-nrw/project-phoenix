package presence

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ProcessSchoolCheckinBatch applies one explicit check-in/out action to a set
// of students (#2359, Sammelauswahl in der Kindersuche).
//
// The batch is set-oriented end to end (review #2372) — it does NOT loop over
// the single-student transition cores:
//
//   - ONE ordered SELECT … FOR UPDATE locks and revalidates every student
//     (unknown/foreign/graduated ids become OK=false skips). Ascending id
//     order is the project-wide student lock convention, so overlapping
//     batches serialize on their first shared row instead of deadlocking, and
//     concurrent graduations block until the batch commits.
//   - The batch timestamp and its attendance date are taken AFTER the locks,
//     from the same instant, and all attendance writes happen in ONE SQL
//     statement. A batch therefore cannot straddle Berlin midnight
//     item-by-item: every item shares one date that is consistent with its
//     write timestamp, and the post-write status refresh reads that same
//     date. (The per-item write loop this replaces could date its early items
//     before midnight and its late items after — review #2372.)
//   - Check-in is one multi-row INSERT … ON CONFLICT DO NOTHING against the
//     open-attendance partial unique index; checkout is one state-checked
//     UPDATE … RETURNING. Both are the same race-safe, idempotent shapes as
//     the single-student cores, and their returned row sets — not any
//     pre-read — are the ground truth for Changed.
//   - Side-effect reads are batched: one open-visit query + one visit-ending
//     UPDATE for checkouts, one status-day query for the auto-clear pass
//     (flag state comes from the already-locked student rows), one device
//     resolution, and batch forms of the slot-attendance mirror. Only the
//     writes for individually-flagged students (sick/excused history rows)
//     stay per student — they are rare and genuinely per-child.
//   - SSE is ONE aggregated fan-out: bulk_student_checkin / _checkout per
//     affected topic plus a single dashboard refresh (the #848 shape),
//     instead of up to 1,000 per-student event pairs.
//
// Failure semantics: the caller runs the batch inside one tenant transaction,
// so an unexpected write error must fail the entire call — earlier writes
// would never commit, and reporting them per-student as OK would lie. Only
// conditions detected without poisoning the transaction (unknown/foreign id,
// graduation, both revalidated under the row lock) become OK=false items.
func (s *service) ProcessSchoolCheckinBatch(ctx context.Context, studentIDs []int64, staffID int64, action string) (*SchoolCheckinBatchResult, error) {
	var result *SchoolCheckinBatchResult
	err := s.runInSessionTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = s.processSchoolCheckinBatch(txCtx, studentIDs, staffID, action)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) processSchoolCheckinBatch(
	ctx context.Context,
	studentIDs []int64,
	staffID int64,
	action string,
) (*SchoolCheckinBatchResult, error) {
	if action != SchoolCheckinActionIn && action != SchoolCheckinActionOut {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: fmt.Errorf("unknown action %q", action)}
	}

	writeOrder, locked, err := s.lockSchoolCheckinStudents(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	outcomes, actionable, actionableStudents := schoolCheckinCandidates(writeOrder, locked, timezone.TodayDate())

	// ONE instant for the whole batch, taken AFTER the row locks: every
	// competing attendance opener locks the student row first, so any
	// check-in this batch can observe committed before the locks were
	// granted — its check_in_time predates now, and a checkout stamped with
	// now can never violate chk_checkin_before_checkout. The attendance
	// date derives from the same instant, so date and timestamp cannot
	// disagree, and because all writes below run in single statements the
	// batch cannot straddle a Berlin midnight item-by-item (review #2372).
	now := s.now()
	day := timezone.DateFromTime(now)

	writes := schoolCheckinWrites{changed: make(map[int64]bool, len(actionable))}
	if len(actionable) > 0 {
		if err := s.applySchoolCheckinAction(ctx, action, actionable, actionableStudents, staffID, now, day, &writes); err != nil {
			return nil, err
		}
	}

	for _, studentID := range actionable {
		outcomes[studentID] = SchoolCheckinBatchItem{StudentID: studentID, OK: true, Changed: writes.changed[studentID]}
	}

	// One staff presence auto-stamp for the whole batch (#1439). Every write
	// above acts for the same staff member, so stamping per student would
	// only repeat the identical EnsureCheckedIn work-session lookup once per
	// child, up to the full batch cap. Best-effort exactly like the
	// single-student path: a failure is logged and never fails the batch.
	if writes.stampPresence {
		s.ensureStaffPresence(ctx, staffID, s.attendanceStampSource(ctx))
	}

	// Back-fill the final status for every actionable student (changed or
	// race-absorbed alike) from ONE post-write attendance query. It runs
	// inside the same tenant transaction as the writes and reads the batch
	// date — the same date every write above used, so the refresh can never
	// look at a different day than the writes (review #2372).
	if len(actionable) > 0 {
		if err := s.refreshSchoolCheckinStatuses(ctx, actionable, day, outcomes); err != nil {
			return nil, err
		}
	}

	s.registerSchoolCheckinBatchBroadcast(ctx, action, locked, writes.changed, writes.endedVisits)
	s.trackSchoolCheckinBatchEvent(ctx, action, len(writes.changed))

	return schoolCheckinBatchResult(studentIDs, outcomes), nil
}

// lockSchoolCheckinStudents locks the batch's students in the canonical
// lock/write order: ascending, deduplicated (see the ProcessSchoolCheckinBatch
// doc comment). The ONE ordered batch lock doubles as id resolution AND
// graduation revalidation: rows are locked before any state is read or
// written, so a concurrent grade-transition apply (#405) blocks here instead of
// racing the writes. Missing ids (unknown, foreign, or deleted) and alumni and
// children whose care has ended become OK=false skips.
func (s *service) lockSchoolCheckinStudents(ctx context.Context, studentIDs []int64) ([]int64, map[int64]*StudentRecord, error) {
	writeOrder := append([]int64(nil), studentIDs...)
	slices.Sort(writeOrder)
	writeOrder = slices.Compact(writeOrder)

	locked, err := s.StudentRepo.FindByIDsForUpdate(ctx, writeOrder)
	if err != nil {
		return nil, nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}
	return writeOrder, locked, nil
}

// schoolCheckinCandidates splits the locked batch into the students the
// action applies to and the skipped ones, which get a not-OK outcome. A
// missing id (unknown, foreign or deleted), an alumnus and a child whose care
// has ended are skipped; the batch writes nothing for them (#2487).
func schoolCheckinCandidates(
	writeOrder []int64,
	locked map[int64]*StudentRecord,
	today timezone.Date,
) (map[int64]SchoolCheckinBatchItem, []int64, []*StudentRecord) {
	outcomes := make(map[int64]SchoolCheckinBatchItem, len(writeOrder))
	actionable := make([]int64, 0, len(writeOrder))
	actionableStudents := make([]*StudentRecord, 0, len(writeOrder))
	for _, studentID := range writeOrder {
		student := locked[studentID]
		if student == nil || student.IsAlumnus() || student.CareEndedOn(today) {
			outcomes[studentID] = SchoolCheckinBatchItem{StudentID: studentID}
			continue
		}
		actionable = append(actionable, studentID)
		actionableStudents = append(actionableStudents, student)
	}
	return outcomes, actionable, actionableStudents
}

// schoolCheckinWrites is what the batch's attendance writes changed: the
// students whose row was written, the room visits a checkout ended, and
// whether the acting staff member counts as on duty.
type schoolCheckinWrites struct {
	changed       map[int64]bool
	endedVisits   []*studentpresence.Visit
	stampPresence bool
}

func (s *service) applySchoolCheckinAction(
	ctx context.Context,
	action string,
	actionable []int64,
	actionableStudents []*StudentRecord,
	staffID int64,
	now time.Time,
	day timezone.Date,
	writes *schoolCheckinWrites,
) error {
	if action == SchoolCheckinActionIn {
		insertedIDs, err := s.applyBatchCheckIn(ctx, actionable, actionableStudents, staffID, now, day)
		if err != nil {
			return err
		}
		for _, id := range insertedIDs {
			writes.changed[id] = true
		}
		// Parity with the single path: every check-in write (inserted or
		// race-absorbed) counts as an on-duty action for the acting staff
		// member (#1439).
		writes.stampPresence = true
		return nil
	}
	closedRows, ended, err := s.applyBatchCheckOut(ctx, actionable, staffID, now, day)
	if err != nil {
		return err
	}
	writes.endedVisits = ended
	for _, row := range closedRows {
		writes.changed[row.StudentID] = true
	}
	// Parity with the single path: an idempotent no-op checkout is not
	// an on-duty action and must not open a work session by itself.
	writes.stampPresence = len(closedRows) > 0
	return nil
}

// refreshSchoolCheckinStatuses back-fills each outcome's final status from
// one post-write status read of the batch day.
func (s *service) refreshSchoolCheckinStatuses(ctx context.Context, actionable []int64, day timezone.Date, outcomes map[int64]SchoolCheckinBatchItem) error {
	rows, err := s.SchoolPresence.ListSchoolStatuses(ctx, actionable, day.String())
	if err != nil {
		return &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}
	for _, row := range rows {
		item := outcomes[row.StudentID]
		item.Status = row.Status
		outcomes[row.StudentID] = item
	}
	return nil
}

// schoolCheckinBatchResult assembles the outcomes in the caller's order,
// first occurrence winning for duplicates.
func schoolCheckinBatchResult(studentIDs []int64, outcomes map[int64]SchoolCheckinBatchItem) *SchoolCheckinBatchResult {
	result := &SchoolCheckinBatchResult{Results: make([]SchoolCheckinBatchItem, 0, len(outcomes))}
	seen := make(map[int64]struct{}, len(outcomes))
	for _, studentID := range studentIDs {
		if _, ok := seen[studentID]; ok {
			continue
		}
		seen[studentID] = struct{}{}
		item := outcomes[studentID]
		result.Results = append(result.Results, item)
		if item.OK {
			result.Succeeded++
		} else {
			result.Failed++
		}
	}
	return result
}

// applyBatchCheckIn writes the whole batch's attendance in one multi-row
// INSERT … ON CONFLICT DO NOTHING (the same partial-unique-index shape as the
// single-student core, so concurrent "in" races are absorbed per row) and
// runs the check-in side effects in batch form. Returns the student ids whose
// row was actually inserted — the Changed ground truth.
func (s *service) applyBatchCheckIn(
	ctx context.Context,
	actionable []int64,
	actionableStudents []*StudentRecord,
	staffID int64,
	now time.Time,
	day timezone.Date,
) ([]int64, error) {
	// ONE device resolution for the whole batch — every row is the same
	// virtual web device (review #2372).
	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	deviceID, err := s.resolveDeviceIDForAttendance(ctx, 0)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: errors.Join(ErrDatabaseOperation, err)}
	}

	tenantID := tenant.FromContext(ctx)
	rows := make([]studentpresence.Attendance, 0, len(actionable))
	for _, studentID := range actionable {
		row := studentpresence.Attendance{
			StudentID:   studentID,
			Date:        day.String(),
			CheckInTime: now,
			CheckedInBy: staffID,
			DeviceID:    deviceID,
		}
		row.TenantID = tenantID
		rows = append(rows, row)
	}
	insertedIDs, err := s.SchoolPresence.EnsureAttendanceBatch(ctx, rows)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}

	// Auto-clear sick/excused/planned statuses for every attempted check-in
	// (inserted and absorbed alike — single-path parity). Reads are batched:
	// the live flags come from the already-locked student rows, the planned
	// rows from one status-day query; only the clears for actually-flagged
	// students write per child (review #2372).
	if err := s.autoClearOnBatchCheckin(ctx, actionable, actionableStudents, now, day); err != nil {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: errors.Join(ErrDatabaseOperation, err)}
	}

	// Binary-mode slot mirror, batched. Only inserted rows mirror: an
	// absorbed check-in means the attendance was already open, and whoever
	// opened it ran the mirror at that check-in's own instant — repeating it
	// with a later timestamp could only mis-stamp a different slot window.
	if mode == PresenceModeBinary && s.AttendanceSyncer != nil && len(insertedIDs) > 0 {
		if err := s.AttendanceSyncer.MirrorCheckInAtBatch(ctx, insertedIDs, now); err != nil {
			return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
		}
	}

	return insertedIDs, nil
}

// applyBatchCheckOut closes the whole batch's open attendance in one
// state-checked UPDATE … RETURNING and ends the batch's open room visits in
// one query pair (lookup + guarded UPDATE), preserving the single-student
// semantics: visit cleanup runs for every actionable student — healing
// orphaned visits even when no attendance row was open — but only for visits
// entered on (or before) the batch day, so a visit the student started after
// a midnight rollover stays open (#895, review #2372).
func (s *service) applyBatchCheckOut(
	ctx context.Context,
	actionable []int64,
	staffID int64,
	now time.Time,
	day timezone.Date,
) ([]studentpresence.Attendance, []*studentpresence.Visit, error) {
	mode, err := s.GetPresenceMode(ctx)
	if err != nil {
		return nil, nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: errors.Join(ErrDatabaseOperation, err)}
	}
	closedRows, err := s.SchoolPresence.CloseAttendance(ctx, studentpresence.AttendanceCheckout{StudentIDs: actionable, Date: day.String(), At: now, StaffID: staffID})
	if err != nil {
		return nil, nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}

	closedVisits, err := s.closeBatchVisits(ctx, actionable, now, day)
	if err != nil {
		return nil, nil, err
	}

	endedVisits := make([]*studentpresence.Visit, 0, len(closedVisits))
	for _, visit := range closedVisits {
		endedVisits = append(endedVisits, &visit)
	}

	if err := s.mirrorBatchCheckOut(ctx, mode, actionable, closedVisits, now); err != nil {
		return nil, nil, err
	}

	return closedRows, endedVisits, nil
}

// closeBatchVisits ends the batch's open room visits entered on or before the
// batch day.
func (s *service) closeBatchVisits(ctx context.Context, actionable []int64, now time.Time, day timezone.Date) ([]studentpresence.Visit, error) {
	openVisits, err := s.currentPresenceVisits(ctx, actionable, false)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}
	visitIDs := make([]int64, 0, len(openVisits))
	for _, visit := range openVisits {
		if timezone.DateFromTime(visit.EntryTime).After(day) {
			continue // newer care session after the rollover — leave it open
		}
		visitIDs = append(visitIDs, visit.ID)
	}
	closedVisits, err := s.SchoolPresence.CloseVisits(ctx, visitIDs, now)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}
	return closedVisits, nil
}

// mirrorBatchCheckOut mirrors the batch checkout into slot attendance.
func (s *service) mirrorBatchCheckOut(ctx context.Context, mode string, actionable []int64, closedVisits []studentpresence.Visit, now time.Time) error {
	if s.AttendanceSyncer == nil {
		return nil
	}
	if mode == PresenceModeBinary {
		// Heal semantics like the single path: every actionable checkout
		// closes its latest open slot, even the idempotent ones. The old
		// same-day guard is structural now — day derives from now, so the
		// mirror can never target a different day than the checkout.
		if err := s.AttendanceSyncer.MirrorCheckOutAtBatch(ctx, actionable, now); err != nil {
			return &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
		}
		return nil
	}
	if len(closedVisits) == 0 {
		return nil
	}
	// Detailed mode mirrors through the ended visits' provenance.
	visits := make([]*studentpresence.Visit, 0, len(closedVisits))
	for i := range closedVisits {
		visits = append(visits, &closedVisits[i])
	}
	if err := s.AttendanceSyncer.MirrorCheckOutForVisits(ctx, visits, now); err != nil {
		return &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}
	return nil
}

// autoClearOnBatchCheckin runs the check-in auto-clear pass for the whole
// batch. Clear modes resolve once, live sick/excused flags are read off the
// already-locked student rows, and the planned/parent status-day rows come
// from ONE query — the per-student FindByID/FindActiveByStudentAndDateRange
// cascade of the single-student helpers never runs here (review #2372). The
// clears themselves stay per flagged student: they carry per-child history
// cascades (status-day upsert + slot release) and flagged students are the
// rare case in a bulk check-in.
func (s *service) autoClearOnBatchCheckin(
	ctx context.Context,
	actionable []int64,
	actionableStudents []*StudentRecord,
	now time.Time,
	day timezone.Date,
) error {
	if err := s.clearBatchLiveFlags(ctx, actionableStudents, now); err != nil {
		return err
	}
	if s.StudentStatusRepo == nil {
		return nil
	}
	return s.clearBatchPlannedStatuses(ctx, actionable, actionableStudents, now, day)
}

// clearBatchLiveFlags clears the sick and excused flags of the locked students
// when the tenant clears them on the next check-in.
func (s *service) clearBatchLiveFlags(ctx context.Context, actionableStudents []*StudentRecord, now time.Time) error {
	sickMode, err := s.resolveSickClearMode(ctx)
	if err != nil {
		return err
	}
	if sickMode == ClearModeNextCheckin {
		for _, student := range actionableStudents {
			if err := s.clearSickFlagOnCheckin(ctx, student, now); err != nil {
				return err
			}
		}
	}
	excusedMode, err := s.resolveExcusedClearMode(ctx)
	if err != nil {
		return err
	}
	if excusedMode != ClearModeNextCheckin {
		return nil
	}
	for _, student := range actionableStudents {
		if err := s.clearExcusedFlagOnCheckin(ctx, student, now); err != nil {
			return err
		}
	}
	return nil
}

// clearBatchPlannedStatuses clears the batch day's planned and parent-reported
// status days from one status-day read.
func (s *service) clearBatchPlannedStatuses(
	ctx context.Context,
	actionable []int64,
	actionableStudents []*StudentRecord,
	now time.Time,
	day timezone.Date,
) error {
	rows, err := s.StudentStatusRepo.FindActiveByStudentIDsAndDate(ctx, actionable, day)
	if err != nil {
		return fmt.Errorf("load planned student statuses: %w", err)
	}
	rowsByStudent := make(map[int64][]*absencerecords.StudentStatusDay, len(actionable))
	for _, row := range rows {
		rowsByStudent[row.StudentID] = append(rowsByStudent[row.StudentID], row)
	}
	if len(rowsByStudent) == 0 {
		return nil
	}
	for _, student := range actionableStudents {
		studentRows := rowsByStudent[student.ID]
		if len(studentRows) == 0 {
			continue
		}
		if err := s.clearPlannedStatusRows(ctx, student.ID, student, studentRows, now); err != nil {
			return err
		}
	}
	return nil
}

// registerSchoolCheckinBatchBroadcast queues ONE aggregated SSE fan-out for
// the whole batch (review #2372), replacing the per-student event pairs of
// the single-student cores with the #848 bulk shape: one bulk event per
// affected educational-group topic, one per active group whose room roster
// changed (checkouts that ended visits), and a single tenant-wide dashboard
// refresh. Bulk events are cache-invalidation triggers carrying student IDs
// only — clients refetch via bulk endpoints, so no names and no attendance
// snapshots travel here.
//
// All display data comes from the already-locked student rows; the emission
// itself is deferred to tenant.RegisterAfterCommit for the same reason as the
// single-student broadcasts (#2113): clients woken before the commit would
// refetch the pre-batch state.
func (s *service) registerSchoolCheckinBatchBroadcast(
	ctx context.Context,
	action string,
	students map[int64]*StudentRecord,
	changed map[int64]bool,
	endedVisits []*studentpresence.Visit,
) {
	if s.Broadcaster == nil {
		return
	}

	// Students to notify about: everyone whose attendance changed, plus (on
	// checkout) everyone whose room visit ended — the same set the single
	// path broadcasts for (a healed orphan visit broadcasts even without an
	// attendance change).
	notify := make(map[int64]struct{}, len(changed)+len(endedVisits))
	for studentID := range changed {
		notify[studentID] = struct{}{}
	}
	for _, visit := range endedVisits {
		notify[visit.StudentID] = struct{}{}
	}
	if len(notify) == 0 {
		return
	}

	checkIn := action != SchoolCheckinActionOut
	eduGroups, allEduGroupIDs := studentsByEducationGroup(notify, students)
	activeGroups := endedVisitsByActiveGroup(endedVisits)

	tenant.RegisterAfterCommit(ctx, func() {
		// One event per active group whose roster changed.
		for groupID, ids := range activeGroups {
			groupIDStr := strconv.FormatInt(groupID, 10)
			realtimeevents.PublishBulkStudentChange(ctx, s.Broadcaster, s.getLogger(), checkIn, groupIDStr, ids, allEduGroupIDs)
		}

		// One event per distinct educational group, carrying only that
		// group's students so each client invalidates the right caches.
		// No active group: this is a roomless attendance change.
		for gid, ids := range eduGroups {
			realtimeevents.PublishBulkStudentChangeToEducationGroup(ctx, s.Broadcaster, s.getLogger(), checkIn, "", gid, ids)
		}

		// Single tenant-wide refresh for the entire batch. The group-specific
		// events above carry the individual roster topics.
		s.broadcastSupervisionRefresh(ctx, "", activeSupervisionReasonStudentMoved, allEduGroupIDs)
	})
}

// studentsByEducationGroup buckets the notified students by educational
// group and lists every such group.
func studentsByEducationGroup(notify map[int64]struct{}, students map[int64]*StudentRecord) (map[int64][]string, []string) {
	eduGroups := make(map[int64][]string)
	for studentID := range notify {
		student := students[studentID]
		if student == nil || student.GroupID == nil {
			continue
		}
		gid := *student.GroupID
		eduGroups[gid] = append(eduGroups[gid], strconv.FormatInt(studentID, 10))
	}
	allEduGroupIDs := make([]string, 0, len(eduGroups))
	for gid := range eduGroups {
		allEduGroupIDs = append(allEduGroupIDs, strconv.FormatInt(gid, 10))
	}
	return eduGroups, allEduGroupIDs
}

// endedVisitsByActiveGroup buckets ended visits by active group for the
// room-roster topics.
func endedVisitsByActiveGroup(endedVisits []*studentpresence.Visit) map[int64][]string {
	activeGroups := make(map[int64][]string)
	for _, visit := range endedVisits {
		if visit.ActiveGroupID <= 0 {
			continue
		}
		activeGroups[visit.ActiveGroupID] = append(
			activeGroups[visit.ActiveGroupID], strconv.FormatInt(visit.StudentID, 10),
		)
	}
	return activeGroups
}

// trackSchoolCheckinBatchEvent emits ONE aggregated product event per batch
// instead of one per changed student (review #2372) — the count travels as a
// property, and "batch: true" separates the shape from single check-ins in
// analytics.
func (s *service) trackSchoolCheckinBatchEvent(ctx context.Context, action string, changedCount int) {
	if changedCount == 0 {
		return
	}
	if action == SchoolCheckinActionIn {
		s.trackProductEvent(ctx, "student_checked_in", map[string]any{
			"method": s.attendanceMethod(ctx),
			"batch":  true,
			"count":  changedCount,
		})
		return
	}
	s.trackProductEvent(ctx, "student_checked_out", map[string]any{
		"method":        s.attendanceMethod(ctx),
		"checkout_type": checkoutTypeWeb,
		"batch":         true,
		"count":         changedCount,
	})
}
