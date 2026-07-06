// Package timetable — WP-B12 gap detection.
//
//	GET /api/timetable/gaps?date=YYYY-MM-DD&date_to=YYYY-MM-DD
//
// Lists instances in the requested window whose non-absent staff count is
// zero. Only today and the future are queryable; ranges > 14 days are
// rejected. Permission: SchedulesRead.
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
}

// GapsResponse is the 200 body for GET /gaps.
type GapsResponse struct {
	From string        `json:"from"`
	To   string        `json:"to"`
	Gaps []GapInstance `json:"gaps"`
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

	// Single GROUP-BY query for non-absent counts across all candidates.
	nonAbsentCounts, err := rs.TimetableData.CountNonAbsentInstanceStaffByInstanceIDs(ctx, candidateIDs)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("count non-absent staff failed", err))
		return
	}

	// Absent-staff count is needed per-instance for the response. An instance
	// marked as a gap (non_absent == 0) still carries information: were there
	// staff assigned but all sick? That signal helps admins triage.
	//
	// NOTE (N+1): one FindByInstanceID per gap. Acceptable today because the
	// gap list is short by definition (most instances have staff). If this
	// grows into a hot path, replace with a second GROUP-BY on
	// is_absent=true, batched across candidateIDs.
	gaps := make([]GapInstance, 0)
	for _, inst := range candidates {
		if nonAbsentCounts[inst.ID] > 0 {
			continue
		}
		// Load the full instance_staff list to compute absent count. Not
		// every gap has absent rows; the count is 0 when no staff was ever
		// assigned.
		rows, err := rs.TimetableData.GetInstanceStaff(ctx, inst.ID)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServerWrap("load instance staff failed", err))
			return
		}
		absentCount := 0
		for _, row := range rows {
			if row.IsAbsent {
				absentCount++
			}
		}
		gaps = append(gaps, GapInstance{
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
		})
	}

	sort.SliceStable(gaps, func(i, j int) bool {
		if gaps[i].Date != gaps[j].Date {
			return gaps[i].Date < gaps[j].Date
		}
		return gaps[i].StartTime < gaps[j].StartTime
	})

	resp := GapsResponse{
		From: from.String(),
		To:   to.String(),
		Gaps: gaps,
	}

	rs.getLogger().Info("timetable gaps",
		slog.String("from", resp.From),
		slog.String("to", resp.To),
		slog.Int("gap_count", len(gaps)),
	)
	common.Respond(w, r, http.StatusOK, resp, "Gaps retrieved")
}
