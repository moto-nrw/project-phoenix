// Package timetable — WP-B12 substitute flow.
//
//	POST /api/timetable/substitute
//
// Body: {absent_staff_id, substitute_staff_id, date}. For every instance_staff
// row of the absent staff on the given date, mark the original is_absent=true
// and insert a substitute row. Idempotent on replay; atomic on 409.
//
// substitute_staff_id is optional (#1840): when omitted the request is
// "absent-only" — the staff is marked absent and each affected position is left
// open (markAbsentOnly). No substitute row is created. Everything below applies
// to the full substitute (both ids present) path.
//
// Atomicity note: the TenantTxMiddleware rolls the request transaction back
// only on 5xx (see tenant/http_middleware.go:40). A 409 rendered mid-handler
// would therefore commit any writes made before the 409. We avoid that with a
// strict two-phase flow:
//
//	Phase A (Dry-Run): classify every target instance without writing.
//	                   A single 409 case aborts the whole request.
//	Phase B (Apply):   only reached when Phase A validates cleanly. All writes
//	                   happen here, so a 409 never commits partial state.
//
// Permission: SchedulesManage. Same tenant tx as other /instances routes.
package timetable

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// substituteRequest is the POST body shape.
type substituteRequest struct {
	AbsentStaffID     int64   `json:"absent_staff_id"`
	SubstituteStaffID int64   `json:"substitute_staff_id"`
	Date              string  `json:"date"`
	Reason            *string `json:"reason,omitempty"` // optional "why" for the absence (#1840)
}

// substituteActionType is the stable per-instance action string returned in
// the response. Callers switch on it rather than parsing messages.
const (
	substituteActionSubstituted       = "substituted"
	substituteActionAlreadySubstitute = "already_substituted"
	substituteActionAlreadyOnInstance = "already_on_instance"
	// Absent-only mode (#1840): substitute_staff_id omitted. The absent staff
	// is marked absent and the position is left open.
	substituteActionMarkedAbsent  = "marked_absent"
	substituteActionAlreadyAbsent = "already_absent"
)

// AffectedInstance is one row in the affected_instances list of the response.
type AffectedInstance struct {
	InstanceID int64  `json:"instance_id"`
	Title      string `json:"title"`
	StartTime  string `json:"start_time"`
	Action     string `json:"action"`
}

// SubstituteResponse is the 200 body.
type SubstituteResponse struct {
	AbsentStaffID     int64                                `json:"absent_staff_id"`
	SubstituteStaffID int64                                `json:"substitute_staff_id"`
	Date              string                               `json:"date"`
	AffectedInstances []AffectedInstance                   `json:"affected_instances"`
	Warnings          []scheduleSvc.SubstituteTimeConflict `json:"warnings"`
}

// plannedOp holds one classified target instance and the decision of what
// Phase B should do with it. Both pointer fields are always populated —
// Phase A pushes an op only after successfully loading the instance and
// classifying the origRow.
type plannedOp struct {
	instance *scheduleModel.ActivityInstance
	origRow  *scheduleModel.InstanceStaff
	action   string
}

// substitute handles POST /api/timetable/substitute.
func (rs *Resource) substitute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// --- Parse + validate the request body --------------------------------
	var req substituteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
		return
	}
	if req.AbsentStaffID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("absent_staff_id is required")))
		return
	}
	// substitute_staff_id is optional (#1840): omitting it (0) marks the staff
	// absent and leaves the position open. A negative id is malformed.
	if req.SubstituteStaffID < 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("substitute_staff_id must be a positive id")))
		return
	}
	if req.SubstituteStaffID > 0 && req.AbsentStaffID == req.SubstituteStaffID {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("absent and substitute staff must differ")))
		return
	}
	if req.Date == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date is required")))
		return
	}
	date, err := berlinDate(req.Date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date format, expected YYYY-MM-DD")))
		return
	}

	// Past dates are rejected. /substitute is a planner tool for "today's staff
	// is out" scenarios; mutating is_absent flags on completed instances would
	// rewrite history. Matches the /gaps policy. Post-hoc correction (if ever
	// required) belongs behind a distinct endpoint or an explicit flag, not
	// this fast-path.
	if date.Before(timezone.TodayDate()) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("'date' must be today or a future date")))
		return
	}

	if rs.TimetableData == nil || rs.PersonService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	// --- 404 checks: both staff must exist in this tenant -----------------
	absentStaff, err := rs.PersonService.GetStaffByID(ctx, req.AbsentStaffID)
	if err != nil {
		if base.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("absent staff not found")))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load absent staff failed", err))
		return
	}
	if absentStaff == nil || absentStaff.ID == 0 {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("absent staff not found")))
		return
	}

	// Absent-only mode (#1840): no substitute given → mark the staff absent
	// across the day and leave every affected position open. Handled by a
	// dedicated helper so the classify/two-phase substitute path below stays
	// exactly as-is.
	if req.SubstituteStaffID == 0 {
		rs.markAbsentOnly(w, r, req, date)
		return
	}

	subStaff, err := rs.PersonService.GetStaffByID(ctx, req.SubstituteStaffID)
	if err != nil {
		if base.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("substitute staff not found")))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load substitute staff failed", err))
		return
	}
	if subStaff == nil || subStaff.ID == 0 {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("substitute staff not found")))
		return
	}

	// --- Load absent staff's same-day assignments -------------------------
	origRows, err := rs.TimetableData.GetInstanceStaffByStaffAndDate(ctx, req.AbsentStaffID, date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load absent assignments failed", err))
		return
	}

	// No rows → empty success response. An absent staff without a scheduled
	// day is not an error; responding 404 would conflate "staff exists?" with
	// "staff has work today?".
	if len(origRows) == 0 {
		rs.getLogger().Info("substitute no-op (no assignments)",
			slog.Int64("absent_staff_id", req.AbsentStaffID),
			slog.Int64("substitute_staff_id", req.SubstituteStaffID),
			slog.String("date", req.Date),
		)
		common.Respond(w, r, http.StatusOK, SubstituteResponse{
			AbsentStaffID:     req.AbsentStaffID,
			SubstituteStaffID: req.SubstituteStaffID,
			Date:              req.Date,
			AffectedInstances: []AffectedInstance{},
			Warnings:          []scheduleSvc.SubstituteTimeConflict{},
		}, "No assignments to substitute")
		return
	}

	// ======================================================================
	// PHASE A — Dry-Run: classify every row, no writes.
	// ======================================================================
	plan := make([]plannedOp, 0, len(origRows))

	for _, orig := range origRows {
		instance, err := rs.TimetableData.GetActivityInstance(ctx, orig.InstanceID)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("load target instance failed", err))
			return
		}
		if instance == nil {
			// Data-corruption branch: instance_staff row references an instance
			// that no longer exists. Aborting is safer than silently skipping.
			common.RenderError(w, r, common.ErrorInternalServer(
				fmt.Errorf("instance_staff %d references missing instance %d", orig.ID, orig.InstanceID)))
			return
		}

		// Skip terminal (completed/cancelled) instances: those are historical
		// staffing that must not be rewritten. A same-day already-finished block
		// is returned by the loader but is not a valid substitute target.
		if !isPlannableInstance(instance) {
			continue
		}

		// All rows of this instance — needed to detect existing substitutes
		// and co-supervisor cases.
		allRows, err := rs.TimetableData.GetInstanceStaff(ctx, instance.ID)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("load instance staff failed", err))
			return
		}

		action, conflictOtherStaff, ok := classifySubstitute(allRows, orig, req.SubstituteStaffID)
		if !ok {
			// Phase A aborts the whole request — no writes have happened yet,
			// so even though the tenant middleware will commit the (empty)
			// tx, no DB state changed. The stable code lets the frontend
			// render the conflict without parsing the German message.
			common.RenderError(w, r, common.ErrorConflictWithCode(
				fmt.Errorf("instance %d has a different substitute already assigned (staff_id=%d); remove the existing substitute first",
					instance.ID, conflictOtherStaff),
				"substitute_conflict",
			))
			return
		}

		plan = append(plan, plannedOp{
			instance: instance,
			origRow:  orig,
			action:   action,
		})
	}

	// ======================================================================
	// PHASE B — Apply writes. Any DB error from here renders 500 → the
	// middleware rolls the tx back. No 4xx path writes anything.
	// ======================================================================
	now := time.Now()
	affected := make([]AffectedInstance, 0, len(plan))
	// Collect active-group IDs we touched to fire SSE updates at the end.
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)

	for _, op := range plan {
		switch op.action {
		case substituteActionAlreadySubstitute:
			// No writes.

		case substituteActionAlreadyOnInstance:
			// Mark only the absent's original row. Existing co-supervisor
			// row stays untouched: is_substitute, is_primary, room_id
			// preserved — the person was independently planned, not a
			// Vertretung, and reports should preserve that distinction.
			op.origRow.IsAbsent = true
			if r := trimReason(req.Reason); r != nil {
				op.origRow.AbsenceReason = r
			}
			if err := rs.TimetableData.UpdateInstanceStaff(ctx, op.origRow); err != nil {
				common.RenderError(w, r, common.ErrorInternalServerWrap("update original staff row failed", err))
				return
			}
			// For active instances, end the absent's supervisor row. Do NOT
			// create a new supervisor — the substitute (co-supervisor) is
			// already an active supervisor.
			if op.instance.Status == scheduleModel.InstanceStatusActive && op.instance.ActiveGroupID != nil {
				if _, err := rs.TimetableData.EndGroupSupervisor(ctx, *op.instance.ActiveGroupID, req.AbsentStaffID); err != nil {
					common.RenderError(w, r, common.ErrorInternalServerWrap("end absent supervisor failed", err))
					return
				}
				activeTouched[*op.instance.ActiveGroupID] = op.instance
			}

		case substituteActionSubstituted:
			op.origRow.IsAbsent = true
			if r := trimReason(req.Reason); r != nil {
				op.origRow.AbsenceReason = r
			}
			if err := rs.TimetableData.UpdateInstanceStaff(ctx, op.origRow); err != nil {
				common.RenderError(w, r, common.ErrorInternalServerWrap("update original staff row failed", err))
				return
			}
			newRow := &scheduleModel.InstanceStaff{
				InstanceID:   op.instance.ID,
				StaffID:      req.SubstituteStaffID,
				RoomID:       op.origRow.RoomID, // inherit room split, if any
				IsPrimary:    false,
				IsSubstitute: true,
				IsAbsent:     false,
			}
			if err := rs.TimetableData.CreateInstanceStaff(ctx, newRow); err != nil {
				common.RenderError(w, r, common.ErrorInternalServerWrap("create substitute staff row failed", err))
				return
			}
			if op.instance.Status == scheduleModel.InstanceStatusActive && op.instance.ActiveGroupID != nil {
				if _, err := rs.TimetableData.EndGroupSupervisor(ctx, *op.instance.ActiveGroupID, req.AbsentStaffID); err != nil {
					common.RenderError(w, r, common.ErrorInternalServerWrap("end absent supervisor failed", err))
					return
				}
				newSup := &activeModel.GroupSupervisor{
					StaffID:   req.SubstituteStaffID,
					GroupID:   *op.instance.ActiveGroupID,
					Role:      "supervisor",
					StartDate: timezone.DateFromTime(now),
				}
				newSup.SetTenantID(tenant.FromContext(ctx))
				if err := rs.TimetableData.CreateGroupSupervisor(ctx, newSup); err != nil {
					common.RenderError(w, r, common.ErrorInternalServerWrap("create substitute supervisor failed", err))
					return
				}
				activeTouched[*op.instance.ActiveGroupID] = op.instance
			}
		}

		affected = append(affected, AffectedInstance{
			InstanceID: op.instance.ID,
			Title:      op.instance.Title,
			StartTime:  op.instance.StartTime.Format("15:04"),
			Action:     op.action,
		})
	}

	// ======================================================================
	// Soft warnings: substitute's OTHER same-day assignments that overlap a
	// target. Only relevant for instances actually substituted or co-cover.
	// Dry-run: no DB writes, just a second Find + comparison.
	// ======================================================================
	warnings, err := rs.buildSubstituteTimeConflicts(ctx, plan, req.SubstituteStaffID, date)
	if err != nil {
		// A warning lookup failure is not a showstopper. Log and continue
		// with an empty warning list — the mutations above are correct
		// independent of the advisory output.
		rs.getLogger().Warn("substitute time-conflict detection failed",
			slog.Int64("substitute_staff_id", req.SubstituteStaffID),
			slog.String("error", err.Error()),
		)
		warnings = nil
	}
	if warnings == nil {
		warnings = []scheduleSvc.SubstituteTimeConflict{}
	}

	// ======================================================================
	// SSE broadcast — fire-and-forget inside the tx, consistent with WP-B9.
	// ======================================================================
	rs.broadcastSubstituteEvents(ctx, activeTouched)

	rs.getLogger().Info("substitute applied",
		slog.Int64("absent_staff_id", req.AbsentStaffID),
		slog.Int64("substitute_staff_id", req.SubstituteStaffID),
		slog.String("date", req.Date),
		slog.Int("affected_count", len(affected)),
		slog.Int("warning_count", len(warnings)),
	)

	common.Respond(w, r, http.StatusOK, SubstituteResponse{
		AbsentStaffID:     req.AbsentStaffID,
		SubstituteStaffID: req.SubstituteStaffID,
		Date:              req.Date,
		AffectedInstances: affected,
		Warnings:          warnings,
	}, "Substitute applied")
}

// markAbsentOnly handles the absent-only branch of POST /substitute (#1840):
// substitute_staff_id was omitted, so we mark the absent staff's same-day
// assignments is_absent=true and leave the positions open. No substitute row is
// created. For active instances the absent's live supervisor row is ended, same
// as the substitute path. There is no 409 case here — marking absent is
// idempotent and conflict-free — so a single write pass is safe (any error is a
// 500 that the tenant middleware rolls back). Reuses the shared SSE broadcast.
func (rs *Resource) markAbsentOnly(w http.ResponseWriter, r *http.Request, req substituteRequest, date timezone.Date) {
	ctx := r.Context()

	origRows, err := rs.TimetableData.GetInstanceStaffByStaffAndDate(ctx, req.AbsentStaffID, date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load absent assignments failed", err))
		return
	}
	if len(origRows) == 0 {
		rs.getLogger().Info("mark-absent no-op (no assignments)",
			slog.Int64("absent_staff_id", req.AbsentStaffID),
			slog.String("date", req.Date),
		)
		common.Respond(w, r, http.StatusOK, SubstituteResponse{
			AbsentStaffID:     req.AbsentStaffID,
			SubstituteStaffID: 0,
			Date:              req.Date,
			AffectedInstances: []AffectedInstance{},
			Warnings:          []scheduleSvc.SubstituteTimeConflict{},
		}, "No assignments to mark absent")
		return
	}

	affected := make([]AffectedInstance, 0, len(origRows))
	activeTouched := make(map[int64]*scheduleModel.ActivityInstance)

	for _, orig := range origRows {
		instance, err := rs.TimetableData.GetActivityInstance(ctx, orig.InstanceID)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("load target instance failed", err))
			return
		}
		if instance == nil {
			common.RenderError(w, r, common.ErrorInternalServer(
				fmt.Errorf("instance_staff %d references missing instance %d", orig.ID, orig.InstanceID)))
			return
		}

		// Skip terminal (completed/cancelled) instances so an already-finished
		// same-day block is never retroactively marked absent. See
		// isPlannableInstance.
		if !isPlannableInstance(instance) {
			continue
		}

		action := substituteActionMarkedAbsent
		if orig.IsAbsent {
			// Already absent — idempotent replay, no write.
			action = substituteActionAlreadyAbsent
		} else {
			orig.IsAbsent = true
			orig.AbsenceReason = trimReason(req.Reason)
			if err := rs.TimetableData.UpdateInstanceStaff(ctx, orig); err != nil {
				common.RenderError(w, r, common.ErrorInternalServerWrap("update original staff row failed", err))
				return
			}
			if instance.Status == scheduleModel.InstanceStatusActive && instance.ActiveGroupID != nil {
				if _, err := rs.TimetableData.EndGroupSupervisor(ctx, *instance.ActiveGroupID, req.AbsentStaffID); err != nil {
					common.RenderError(w, r, common.ErrorInternalServerWrap("end absent supervisor failed", err))
					return
				}
				activeTouched[*instance.ActiveGroupID] = instance
			}
		}

		affected = append(affected, AffectedInstance{
			InstanceID: instance.ID,
			Title:      instance.Title,
			StartTime:  instance.StartTime.Format("15:04"),
			Action:     action,
		})
	}

	rs.broadcastSubstituteEvents(ctx, activeTouched)

	rs.getLogger().Info("mark-absent applied",
		slog.Int64("absent_staff_id", req.AbsentStaffID),
		slog.String("date", req.Date),
		slog.Int("affected_count", len(affected)),
	)

	common.Respond(w, r, http.StatusOK, SubstituteResponse{
		AbsentStaffID:     req.AbsentStaffID,
		SubstituteStaffID: 0,
		Date:              req.Date,
		AffectedInstances: affected,
		Warnings:          []scheduleSvc.SubstituteTimeConflict{},
	}, "Staff marked absent")
}

// trimReason normalizes an optional deviation reason: nil/blank becomes nil,
// and an over-long value is truncated to the shared note ceiling so a single
// oversized field can never bloat a row. The ceiling counts runes, not bytes:
// slicing on a byte offset can split a multi-byte UTF-8 rune, producing an
// invalid string that PostgreSQL rejects. The frontend's maxLength is likewise
// a character count, so a rune ceiling keeps the two limits consistent.
func trimReason(reason *string) *string {
	if reason == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*reason)
	if trimmed == "" {
		return nil
	}
	if utf8.RuneCountInString(trimmed) > understaffedAckNoteMaxLength {
		trimmed = string([]rune(trimmed)[:understaffedAckNoteMaxLength])
	}
	return &trimmed
}

// isPlannableInstance reports whether a substitute/absence write may touch this
// instance. Only planned and active blocks are editable: completed and
// cancelled ones are historical record. GetInstanceStaffByStaffAndDate returns
// every same-day assignment regardless of status, so a staff member with an
// already-finished morning block and a planned afternoon block would otherwise
// have the completed block's staffing rewritten. Mirrors the /gaps candidate
// filter and DeleteUpcomingByStaffID's "keep same-day history" rule.
func isPlannableInstance(instance *scheduleModel.ActivityInstance) bool {
	return instance.Status == scheduleModel.InstanceStatusPlanned ||
		instance.Status == scheduleModel.InstanceStatusActive
}

// classifySubstitute decides the action for a single target instance. Pure
// logic, no DB. Returns (action, conflictingOtherStaffID, ok=false on 409).
//
// allRows: every instance_staff row of the target instance.
// origRow: the absent's row on the target instance.
// subID:   the substitute's staff id.
func classifySubstitute(
	allRows []*scheduleModel.InstanceStaff,
	origRow *scheduleModel.InstanceStaff,
	subID int64,
) (action string, conflictOtherStaff int64, ok bool) {
	// Scan once for the three signals we need.
	var existingSubOfSub *scheduleModel.InstanceStaff
	var existingSubOfOther *scheduleModel.InstanceStaff
	var subAsNonAbsent *scheduleModel.InstanceStaff
	for _, row := range allRows {
		if row.IsSubstitute && row.StaffID == subID {
			existingSubOfSub = row
		}
		if row.IsSubstitute && row.StaffID != subID && !row.IsAbsent {
			existingSubOfOther = row
		}
		if !row.IsSubstitute && row.StaffID == subID && !row.IsAbsent {
			subAsNonAbsent = row
		}
	}

	if origRow.IsAbsent && existingSubOfSub != nil {
		return substituteActionAlreadySubstitute, 0, true
	}
	if origRow.IsAbsent && existingSubOfOther != nil {
		return "", existingSubOfOther.StaffID, false
	}
	if subAsNonAbsent != nil {
		// Substitute is already a co-supervisor on this instance. We cannot
		// insert a second row for the same staff (UNIQUE(instance_id,
		// staff_id) would reject it); semantically the person is already
		// covering. Mark the absent's row and leave the co-supervisor row
		// untouched — it remains is_substitute=false so the reporting layer
		// can distinguish planned co-cover from a Vertretung.
		return substituteActionAlreadyOnInstance, 0, true
	}
	return substituteActionSubstituted, 0, true
}

// buildSubstituteTimeConflicts loads the substitute's OTHER (non-target)
// same-day non-absent assignments and returns overlap warnings. No writes.
func (rs *Resource) buildSubstituteTimeConflicts(
	ctx context.Context,
	plan []plannedOp,
	subID int64,
	date timezone.Date,
) ([]scheduleSvc.SubstituteTimeConflict, error) {
	if len(plan) == 0 {
		return nil, nil
	}
	subRows, err := rs.TimetableData.GetInstanceStaffByStaffAndDate(ctx, subID, date)
	if err != nil {
		return nil, err
	}
	// Target IDs to exclude from "foreign" set.
	targetSet := make(map[int64]bool, len(plan))
	for _, op := range plan {
		targetSet[op.instance.ID] = true
	}

	foreignIDs := make([]int64, 0, len(subRows))
	for _, row := range subRows {
		if row.IsAbsent {
			continue
		}
		if targetSet[row.InstanceID] {
			continue
		}
		foreignIDs = append(foreignIDs, row.InstanceID)
	}
	if len(foreignIDs) == 0 {
		return nil, nil
	}
	foreigns := make([]scheduleSvc.SubstituteConflictInstance, 0, len(foreignIDs))
	for _, fid := range foreignIDs {
		inst, err := rs.TimetableData.GetActivityInstance(ctx, fid)
		if err != nil {
			return nil, err
		}
		if inst == nil {
			continue
		}
		foreigns = append(foreigns, toConflictInstance(inst))
	}
	targets := make([]scheduleSvc.SubstituteConflictInstance, 0, len(plan))
	for _, op := range plan {
		if op.action == substituteActionAlreadySubstitute {
			// Already substituted earlier — we didn't write anything; still
			// safe to compare the substitute's existing foreign assignments.
			continue
		}
		// already_on_instance is intentionally included: no new substitute
		// row is created, but the co-supervisor's pre-existing overlaps with
		// other same-day assignments remain relevant information for the
		// admin. Pruning them would hide a truthful signal.
		targets = append(targets, toConflictInstance(op.instance))
	}
	return scheduleSvc.DetectSubstituteTimeConflicts(targets, foreigns), nil
}

// toConflictInstance converts an ActivityInstance's TIME columns (year-0000
// time.Time after bun decode) into the minutes-since-midnight form expected
// by the conflict helper.
func toConflictInstance(inst *scheduleModel.ActivityInstance) scheduleSvc.SubstituteConflictInstance {
	return scheduleSvc.SubstituteConflictInstance{
		ID:        inst.ID,
		StartMin:  scheduleSvc.MinutesOfTime(inst.StartTime.Hour(), inst.StartTime.Minute()),
		EndMin:    scheduleSvc.MinutesOfTime(inst.EndTime.Hour(), inst.EndTime.Minute()),
		StartHHMM: inst.StartTime.Format("15:04"),
	}
}

// broadcastSubstituteEvents queues one activity_update per affected active
// group after the surrounding tenant transaction commits.
func (rs *Resource) broadcastSubstituteEvents(
	ctx context.Context,
	touched map[int64]*scheduleModel.ActivityInstance,
) {
	if rs.Broadcaster == nil || len(touched) == 0 {
		return
	}
	tenantID := tenant.FromContext(ctx)
	for activeGroupID, inst := range touched {
		activeGroupIDStr := fmt.Sprintf("%d", activeGroupID)
		instanceIDStr := fmt.Sprintf("%d", inst.ID)
		instanceDate := inst.Date.Format(dateLayout)
		instanceStart := inst.StartTime.Format("15:04:05")
		data := realtime.EventData{
			InstanceID:        &instanceIDStr,
			InstanceDate:      &instanceDate,
			InstanceStartTime: &instanceStart,
		}
		event := realtime.NewEvent(realtime.EventActivityUpdate, activeGroupIDStr, data)
		tenant.RegisterAfterCommit(ctx, func() {
			if err := rs.Broadcaster.BroadcastToGroup(tenantID, activeGroupIDStr, event); err != nil {
				rs.getLogger().Warn("SSE substitute broadcast failed",
					slog.String("active_group_id", activeGroupIDStr),
					slog.String("error", err.Error()),
				)
			}
		})
	}
}
