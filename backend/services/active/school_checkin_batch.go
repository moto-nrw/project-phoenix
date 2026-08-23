package active

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// School check-in action values shared by the single and batch web endpoints.
const (
	SchoolCheckinActionIn  = "in"
	SchoolCheckinActionOut = "out"
)

// SchoolCheckinBatchItem is the per-student outcome of a batch call. OK=false
// means the id was not actionable (unknown, another tenant's student —
// invisible through the tenant filter —, or graduated); those students are
// skipped while the rest of the batch proceeds. Status carries the final
// attendance status for OK items so callers need no follow-up fetch.
type SchoolCheckinBatchItem struct {
	StudentID int64
	OK        bool
	Changed   bool
	Status    string
}

// SchoolCheckinBatchResult aggregates a batch call. Results preserve the
// caller's id order (first occurrence wins for duplicates), independent of the
// canonical write order used internally.
type SchoolCheckinBatchResult struct {
	Results   []SchoolCheckinBatchItem
	Succeeded int
	Failed    int
}

// IsSchoolCheckinNoop reports whether the requested action is already
// satisfied by the student's current attendance status ("on_yard" counts as
// present). Pure decision logic, exported for the single-student handler's
// short-circuit. The batch below deliberately does NOT use it: skipping a
// write based on a pre-read status races a concurrent transition — the
// state-checked writes themselves are the only trustworthy no-op detectors.
func IsSchoolCheckinNoop(action, currentStatus string) bool {
	alreadyPresent := currentStatus == "checked_in" || currentStatus == "on_yard"
	alreadyAbsent := currentStatus == "not_checked_in" || currentStatus == "checked_out"
	switch action {
	case SchoolCheckinActionIn:
		return alreadyPresent
	case SchoolCheckinActionOut:
		return alreadyAbsent
	default:
		return false
	}
}

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
func (s *service) ProcessSchoolCheckinBatch(
	ctx context.Context,
	studentIDs []int64,
	staffID int64,
	action string,
) (*SchoolCheckinBatchResult, error) {
	if action != SchoolCheckinActionIn && action != SchoolCheckinActionOut {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: fmt.Errorf("unknown action %q", action)}
	}

	// Canonical lock/write order: ascending, deduplicated (see doc comment).
	writeOrder := append([]int64(nil), studentIDs...)
	slices.Sort(writeOrder)
	writeOrder = slices.Compact(writeOrder)

	// ONE ordered batch lock doubles as id resolution AND graduation
	// revalidation: rows are locked before any state is read or written, so a
	// concurrent grade-transition apply (#405) blocks here instead of racing
	// the writes below. Missing ids (unknown, foreign, or deleted) and
	// alumni and children whose care has ended become OK=false skips.
	locked, err := s.StudentRepo.FindByIDsForUpdate(ctx, writeOrder)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}

	outcomes := make(map[int64]SchoolCheckinBatchItem, len(writeOrder))
	actionable := make([]int64, 0, len(writeOrder))
	actionableStudents := make([]*userModels.Student, 0, len(writeOrder))
	today := timezone.TodayDate()
	for _, studentID := range writeOrder {
		student := locked[studentID]
		// A child whose care has ended is skipped exactly like an alumnus:
		// the batch reports them as not-OK and writes nothing (#2487).
		if student == nil || student.IsAlumnus() || student.CareEndedOn(today) {
			outcomes[studentID] = SchoolCheckinBatchItem{StudentID: studentID}
			continue
		}
		actionable = append(actionable, studentID)
		actionableStudents = append(actionableStudents, student)
	}

	// ONE instant for the whole batch, taken AFTER the row locks: every
	// competing attendance opener locks the student row first, so any
	// check-in this batch can observe committed before the locks were
	// granted — its check_in_time predates now, and a checkout stamped with
	// now can never violate chk_checkin_before_checkout. The attendance
	// date derives from the same instant, so date and timestamp cannot
	// disagree, and because all writes below run in single statements the
	// batch cannot straddle a Berlin midnight item-by-item (review #2372).
	now := time.Now()
	day := timezone.DateFromTime(now)

	changed := make(map[int64]bool, len(actionable))
	var endedVisits []*active.Visit
	stampPresence := false
	if len(actionable) > 0 {
		switch action {
		case SchoolCheckinActionIn:
			insertedIDs, inErr := s.applyBatchCheckIn(ctx, actionable, actionableStudents, staffID, now, day)
			if inErr != nil {
				return nil, inErr
			}
			for _, id := range insertedIDs {
				changed[id] = true
			}
			// Parity with the single path: every check-in write (inserted or
			// race-absorbed) counts as an on-duty action for the acting staff
			// member (#1439).
			stampPresence = true
		case SchoolCheckinActionOut:
			closedRows, ended, outErr := s.applyBatchCheckOut(ctx, actionable, staffID, now, day)
			if outErr != nil {
				return nil, outErr
			}
			endedVisits = ended
			for _, row := range closedRows {
				changed[row.StudentID] = true
			}
			// Parity with the single path: an idempotent no-op checkout is not
			// an on-duty action and must not open a work session by itself.
			stampPresence = len(closedRows) > 0
		}
	}

	for _, studentID := range actionable {
		outcomes[studentID] = SchoolCheckinBatchItem{StudentID: studentID, OK: true, Changed: changed[studentID]}
	}

	// One staff presence auto-stamp for the whole batch (#1439). Every write
	// above acts for the same staff member, so stamping per student would
	// only repeat the identical EnsureCheckedIn work-session lookup once per
	// child, up to the full batch cap. Best-effort exactly like the
	// single-student path: a failure is logged and never fails the batch.
	if stampPresence {
		s.ensureStaffPresence(ctx, staffID, attendanceStampSource(ctx))
	}

	// Back-fill the final status for every actionable student (changed or
	// race-absorbed alike) from ONE post-write attendance query. It runs
	// inside the same tenant transaction as the writes and reads the batch
	// date — the same date every write above used, so the refresh can never
	// look at a different day than the writes (review #2372).
	if len(actionable) > 0 {
		rows, refreshErr := s.AttendanceRepo.FindForDateByStudentIDs(ctx, day, actionable)
		if refreshErr != nil {
			return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: refreshErr}
		}
		// Rows are ordered check_in_time ASC per student — keep the latest.
		latest := make(map[int64]*active.Attendance, len(actionable))
		for _, row := range rows {
			latest[row.StudentID] = row
		}
		for _, studentID := range actionable {
			item := outcomes[studentID]
			if row := latest[studentID]; row != nil {
				item.Status = deriveAttendanceStatus(row)
			} else {
				// No row on the batch date: an idempotent checkout of a
				// student who never checked in today. Mirrors the missing-row
				// semantics of GetStudentsAttendanceStatuses.
				item.Status = "not_checked_in"
			}
			outcomes[studentID] = item
		}
	}

	s.registerSchoolCheckinBatchBroadcast(ctx, action, locked, changed, endedVisits)
	s.trackSchoolCheckinBatchEvent(ctx, action, len(changed))

	// Assemble in the caller's order, first occurrence winning for duplicates.
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
	return result, nil
}

// applyBatchCheckIn writes the whole batch's attendance in one multi-row
// INSERT … ON CONFLICT DO NOTHING (the same partial-unique-index shape as the
// single-student core, so concurrent "in" races are absorbed per row) and
// runs the check-in side effects in batch form. Returns the student ids whose
// row was actually inserted — the Changed ground truth.
func (s *service) applyBatchCheckIn(
	ctx context.Context,
	actionable []int64,
	actionableStudents []*userModels.Student,
	staffID int64,
	now time.Time,
	day timezone.Date,
) ([]int64, error) {
	// ONE device resolution for the whole batch — every row is the same
	// virtual web device (review #2372).
	deviceID := s.resolveDeviceIDForAttendance(ctx, 0)

	tenantID := tenant.FromContext(ctx)
	rows := make([]*active.Attendance, 0, len(actionable))
	for _, studentID := range actionable {
		row := &active.Attendance{
			StudentID:   studentID,
			Date:        day,
			CheckInTime: now,
			CheckedInBy: staffID,
			DeviceID:    deviceID,
		}
		row.SetTenantID(tenantID)
		rows = append(rows, row)
	}
	insertedIDs, err := s.AttendanceRepo.CreateIfNoOpenForTodayBatch(ctx, rows)
	if err != nil {
		return nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}

	// Auto-clear sick/excused/planned statuses for every attempted check-in
	// (inserted and absorbed alike — single-path parity). Reads are batched:
	// the live flags come from the already-locked student rows, the planned
	// rows from one status-day query; only the clears for actually-flagged
	// students write per child (review #2372).
	s.autoClearOnBatchCheckin(ctx, actionable, actionableStudents, now, day)

	// Binary-mode slot mirror, batched. Only inserted rows mirror: an
	// absorbed check-in means the attendance was already open, and whoever
	// opened it ran the mirror at that check-in's own instant — repeating it
	// with a later timestamp could only mis-stamp a different slot window.
	if s.GetPresenceMode(ctx) == "binary" && s.AttendanceSyncer != nil && len(insertedIDs) > 0 {
		s.AttendanceSyncer.MirrorCheckInAtBatch(ctx, insertedIDs, now)
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
) ([]*active.Attendance, []*active.Visit, error) {
	closedRows, err := s.AttendanceRepo.CloseOpenForDateByStudentIDs(ctx, actionable, now, day, staffID)
	if err != nil {
		return nil, nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}

	openVisits, err := s.VisitRepo.GetCurrentByStudentIDs(ctx, actionable)
	if err != nil {
		return nil, nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}
	visitIDs := make([]int64, 0, len(openVisits))
	for _, visit := range openVisits {
		if timezone.DateFromTime(visit.EntryTime).After(day) {
			continue // newer care session after the rollover — leave it open
		}
		visitIDs = append(visitIDs, visit.ID)
	}
	endedVisits, err := s.VisitRepo.EndVisitsByIDs(ctx, visitIDs, now)
	if err != nil {
		return nil, nil, &ActiveError{Op: "ProcessSchoolCheckinBatch", Err: err}
	}

	if s.AttendanceSyncer != nil {
		if s.GetPresenceMode(ctx) == "binary" {
			// Heal semantics like the single path: every actionable checkout
			// closes its latest open slot, even the idempotent ones. The old
			// same-day guard is structural now — day derives from now, so the
			// mirror can never target a different day than the checkout.
			s.AttendanceSyncer.MirrorCheckOutAtBatch(ctx, actionable, now)
		} else if len(endedVisits) > 0 {
			// Detailed mode mirrors through the ended visits' provenance.
			s.AttendanceSyncer.MirrorCheckOutForVisits(ctx, endedVisits, now)
		}
	}

	return closedRows, endedVisits, nil
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
	actionableStudents []*userModels.Student,
	now time.Time,
	day timezone.Date,
) {
	if s.resolveClearMode(ctx, configModel.KeySickClearMode, configModel.ClearModeNextCheckin) == configModel.ClearModeNextCheckin {
		for _, student := range actionableStudents {
			s.clearSickFlagOnCheckin(ctx, student, now)
		}
	}
	if s.resolveClearMode(ctx, configModel.KeyExcusedClearMode, configModel.ClearModeEndOfDay) == configModel.ClearModeNextCheckin {
		for _, student := range actionableStudents {
			s.clearExcusedFlagOnCheckin(ctx, student, now)
		}
	}

	if s.StudentStatusRepo == nil {
		return
	}
	rows, err := s.StudentStatusRepo.FindActiveByStudentIDsAndDate(ctx, actionable, day)
	if err != nil {
		s.getLogger().Warn("failed to load planned student status days on batch check-in",
			slog.String("error", err.Error()),
		)
		return
	}
	rowsByStudent := make(map[int64][]*active.StudentStatusDay, len(actionable))
	for _, row := range rows {
		rowsByStudent[row.StudentID] = append(rowsByStudent[row.StudentID], row)
	}
	if len(rowsByStudent) == 0 {
		return
	}
	for _, student := range actionableStudents {
		studentRows := rowsByStudent[student.ID]
		if len(studentRows) == 0 {
			continue
		}
		s.clearPlannedStatusRows(ctx, student.ID, student, studentRows, now)
	}
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
	students map[int64]*userModels.Student,
	changed map[int64]bool,
	endedVisits []*active.Visit,
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

	eventType := realtime.EventBulkStudentCheckIn
	if action == SchoolCheckinActionOut {
		eventType = realtime.EventBulkStudentCheckOut
	}

	// Bucket the notified students by educational group; keep one student per
	// group so broadcastToEducationalGroup can derive the edu:{id} topic.
	eduGroups := make(map[int64][]string)
	eduReps := make(map[int64]*userModels.Student)
	for studentID := range notify {
		student := students[studentID]
		if student == nil || student.GroupID == nil {
			continue
		}
		gid := *student.GroupID
		eduGroups[gid] = append(eduGroups[gid], strconv.FormatInt(studentID, 10))
		if eduReps[gid] == nil {
			eduReps[gid] = student
		}
	}
	allEduGroupIDs := make([]string, 0, len(eduGroups))
	for gid := range eduGroups {
		allEduGroupIDs = append(allEduGroupIDs, strconv.FormatInt(gid, 10))
	}

	// Bucket ended visits by active group for the room-roster topics.
	activeGroups := make(map[int64][]string)
	for _, visit := range endedVisits {
		if visit.ActiveGroupID <= 0 {
			continue
		}
		activeGroups[visit.ActiveGroupID] = append(
			activeGroups[visit.ActiveGroupID], strconv.FormatInt(visit.StudentID, 10),
		)
	}

	tenant.RegisterAfterCommit(ctx, func() {
		// One event per active group whose roster changed.
		for groupID, ids := range activeGroups {
			studentIDs := ids
			groupIDStr := strconv.FormatInt(groupID, 10)
			data := realtime.EventData{StudentIDs: &studentIDs}
			if len(allEduGroupIDs) > 0 {
				data.GroupIDs = &allEduGroupIDs
			}
			event := realtime.NewEvent(eventType, groupIDStr, data)
			s.broadcastWithLogging(ctx, groupIDStr, "", event, string(eventType))
		}

		// One event per distinct educational group, carrying only that
		// group's students so each client invalidates the right caches.
		for gid, ids := range eduGroups {
			studentIDs := ids
			eduGroupID := []string{strconv.FormatInt(gid, 10)}
			event := realtime.NewEvent(
				eventType,
				"", // no active group — roomless attendance change
				realtime.EventData{StudentIDs: &studentIDs, GroupIDs: &eduGroupID},
			)
			s.broadcastToEducationalGroup(ctx, eduReps[gid], event)
		}

		// Single tenant-wide refresh for the entire batch. The group-specific
		// events above carry the individual roster topics.
		s.broadcastSupervisionRefresh(ctx, "", activeSupervisionReasonStudentMoved, allEduGroupIDs)
	})
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
			"method": attendanceMethod(ctx),
			"batch":  true,
			"count":  changedCount,
		})
		return
	}
	s.trackProductEvent(ctx, "student_checked_out", map[string]any{
		"method":        attendanceMethod(ctx),
		"checkout_type": checkoutTypeWeb,
		"batch":         true,
		"count":         changedCount,
	})
}
