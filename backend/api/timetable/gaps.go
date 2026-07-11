// Package timetable — WP-B12 gap detection.
//
//	GET /api/timetable/gaps?date=YYYY-MM-DD&date_to=YYYY-MM-DD
//
// Lists instances in the requested window that are understaffed: nobody
// present, OR fewer people present than the number of planned positions (#1840
// — a single position deliberately left unfilled still counts as a shortfall).
// Only today and the future are queryable; ranges > 14 days are rejected.
// Permission: SchedulesRead.
package timetable

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// maxGapRangeDays caps the /gaps window at 14 inclusive days, mirroring the
// /week constant. Longer horizons have no frontend consumer today.
const maxGapRangeDays = 14

// GapInstance is one row in the gaps response.
type GapInstance struct {
	InstanceID         int64  `json:"instance_id"`
	Date               string `json:"date"`
	Title              string `json:"title"`
	StartTime          string `json:"start_time"`
	EndTime            string `json:"end_time"`
	RoomID             int64  `json:"room_id"`
	Status             string `json:"status"`
	AssignedStaffCount int    `json:"assigned_staff_count"`
	AbsentStaffCount   int    `json:"absent_staff_count"`
	// PresentStaffCount is the non-absent count (planned people still there plus
	// any covering substitute); PlannedStaffCount is the base-plan positions
	// (non-substitute rows). A partial shortfall has 0 < present < planned.
	PresentStaffCount int     `json:"present_staff_count"`
	PlannedStaffCount int     `json:"planned_staff_count"`
	UnderstaffedNote  *string `json:"understaffed_note,omitempty"`
}

// GapsResponse is the 200 body for GET /gaps.
//
// Gaps and Acknowledged partition the understaffed instances: Gaps are the open
// shortfalls that still need filling, Acknowledged are the ones an admin
// deliberately left understaffed (understaffed_ack, #1840). The shortfall stays
// visible in both — Acknowledged just moves out of the "needs action" list.
type GapsResponse struct {
	From         string        `json:"from"`
	To           string        `json:"to"`
	Gaps         []GapInstance `json:"gaps"`
	Acknowledged []GapInstance `json:"acknowledged"`
}

// getGaps handles GET /api/timetable/gaps.
func (rs *Resource) getGaps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()

	dateStr := q.Get("date")
	if dateStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date is required")))
		return
	}
	from, err := berlinDate(dateStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date format, expected YYYY-MM-DD")))
		return
	}

	// date_to defaults to date — a single-day query is a window of one.
	toStr := q.Get("date_to")
	to := from
	if toStr != "" {
		to, err = berlinDate(toStr)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date_to format, expected YYYY-MM-DD")))
			return
		}
	}

	if from.After(to) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("'date' must be before or equal to 'date_to'")))
		return
	}
	if inclusiveDayCount(from, to) > maxGapRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			fmt.Errorf("date range exceeds maximum of %d days", maxGapRangeDays)))
		return
	}

	// Past dates are rejected. The gap view is a planning tool — closed days
	// are historical record, not a gap to fill. Comparison uses the Berlin
	// calendar "today" so a 23:00 UTC request still considers the local date
	// correctly.
	if from.Before(timezone.TodayDate()) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("'date' must be today or a future date")))
		return
	}

	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	instances, err := rs.TimetableData.GetActivityInstancesByDateRange(ctx, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load instances failed", err))
		return
	}

	// Only planned and active instances are candidates. completed/cancelled
	// are history — no point surfacing them as gaps to fill.
	candidates := make([]*scheduleModel.ActivityInstance, 0, len(instances))
	candidateIDs := make([]int64, 0, len(instances))
	for _, inst := range instances {
		if inst.Status != scheduleModel.InstanceStatusPlanned &&
			inst.Status != scheduleModel.InstanceStatusActive {
			continue
		}
		candidates = append(candidates, inst)
		candidateIDs = append(candidateIDs, inst.ID)
	}

	// One bulk query for every candidate's staff rows, grouped in memory. This
	// replaces the old non-absent GROUP-BY plus per-gap N+1 load: partial
	// shortfalls are now common (any block with one unreplaced absence), so the
	// gap list is no longer short by definition and per-gap round-trips would
	// not scale.
	allRows, err := rs.TimetableData.GetInstanceStaffByInstanceIDs(ctx, candidateIDs)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load instance staff failed", err))
		return
	}
	rowsByInstance := make(map[int64][]*scheduleModel.InstanceStaff, len(candidateIDs))
	for _, row := range allRows {
		rowsByInstance[row.InstanceID] = append(rowsByInstance[row.InstanceID], row)
	}

	// Gaps are open shortfalls that still need action; acknowledged shortfalls
	// are ones an admin deliberately left understaffed (#1840) and are
	// partitioned out so they stop nagging while remaining visible.
	gaps := make([]GapInstance, 0)
	acknowledged := make([]GapInstance, 0)
	for _, inst := range candidates {
		rows := rowsByInstance[inst.ID]
		if !scheduleSvc.IsUnderstaffed(rows) {
			continue
		}
		absentCount, presentCount, plannedCount := 0, 0, 0
		for _, row := range rows {
			if row.IsAbsent {
				absentCount++
			} else {
				presentCount++
			}
			if !row.IsSubstitute {
				plannedCount++
			}
		}
		gi := GapInstance{
			InstanceID: inst.ID,
			Date:       inst.Date.String(),
			Title:      inst.Title,
			StartTime:  inst.StartTime.Format("15:04"),
			EndTime:    inst.EndTime.Format("15:04"),
			RoomID:     inst.RoomID,
			Status:     inst.Status,
			// AssignedStaffCount is the total instance_staff count — present
			// and absent combined. Matches the PR #1303 contract: an instance
			// with two staff, both is_absent=true, reports assigned=2,absent=2
			// so admins see "staff were planned, nobody's covering today".
			AssignedStaffCount: len(rows),
			AbsentStaffCount:   absentCount,
			PresentStaffCount:  presentCount,
			PlannedStaffCount:  plannedCount,
		}
		if inst.UnderstaffedAck {
			gi.UnderstaffedNote = inst.UnderstaffedNote
			acknowledged = append(acknowledged, gi)
		} else {
			gaps = append(gaps, gi)
		}
	}

	sortGapInstances(gaps)
	sortGapInstances(acknowledged)

	resp := GapsResponse{
		From:         from.String(),
		To:           to.String(),
		Gaps:         gaps,
		Acknowledged: acknowledged,
	}

	rs.getLogger().Info("timetable gaps",
		slog.String("from", resp.From),
		slog.String("to", resp.To),
		slog.Int("gap_count", len(gaps)),
		slog.Int("acknowledged_count", len(acknowledged)),
	)
	common.Respond(w, r, http.StatusOK, resp, "Gaps retrieved")
}

// sortGapInstances orders a gap bucket by date then start time, stably.
func sortGapInstances(items []GapInstance) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Date != items[j].Date {
			return items[i].Date < items[j].Date
		}
		return items[i].StartTime < items[j].StartTime
	})
}
