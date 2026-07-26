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
	"log/slog"
	"net/http"
	"sort"

	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

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
	from, to, ok := parseTodayFutureDateRange(w, r)
	if !ok {
		return
	}

	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	understaffed, err := rs.TimetableData.ComputeGaps(r.Context(), from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("compute gaps failed", err))
		return
	}

	// Partition the understaffed instances: open gaps still need filling;
	// acknowledged shortfalls are ones an admin deliberately left understaffed
	// (#1840) and stop nagging while remaining visible.
	gaps := make([]GapInstance, 0)
	acknowledged := make([]GapInstance, 0)
	for _, u := range understaffed {
		gi := mapGapInstance(u)
		if u.Instance.UnderstaffedAck {
			gi.UnderstaffedNote = u.Instance.UnderstaffedNote
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

// mapGapInstance lifts a computed understaffed instance into the wire shape.
// AssignedStaffCount is the total instance_staff count (present + absent), per
// the PR #1303 contract: an instance with two staff both absent reports
// assigned=2,absent=2 so admins see "staff were planned, nobody's covering".
func mapGapInstance(u scheduleSvc.UnderstaffedInstance) GapInstance {
	inst := u.Instance
	return GapInstance{
		InstanceID:         inst.ID,
		Date:               inst.Date.String(),
		Title:              inst.Title,
		StartTime:          inst.StartTime.Format("15:04"),
		EndTime:            inst.EndTime.Format("15:04"),
		RoomID:             inst.RoomID,
		Status:             inst.Status,
		AssignedStaffCount: u.AssignedStaffCount,
		AbsentStaffCount:   u.AbsentStaffCount,
		PresentStaffCount:  u.PresentStaffCount,
		PlannedStaffCount:  u.PlannedStaffCount,
	}
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
