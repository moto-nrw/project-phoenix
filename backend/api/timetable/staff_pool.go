// Package timetable — staff availability pool for one block (#1884).
//
//	GET /api/timetable/instances/{id}/staff-pool
//
// Returns every staff member categorized against the block's time window:
// already on the block, absent that day, planned on another overlapping block
// (the move sources), free on shift, or not on shift at all. Read-only; the
// atomic move endpoint consumes the same facts.
//
// Permission: SchedulesRead.
package timetable

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// StaffPoolAssignmentResponse is one overlapping same-day assignment of a
// pool member (the move source the frontend offers).
type StaffPoolAssignmentResponse struct {
	InstanceID   int64  `json:"instance_id"`
	Title        string `json:"title"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	IsSubstitute bool   `json:"is_substitute"`
}

// StaffPoolEntryResponse is one categorized staff member.
type StaffPoolEntryResponse struct {
	StaffID       int64                         `json:"staff_id"`
	DisplayName   string                        `json:"display_name"`
	Category      string                        `json:"category"`
	OnShift       bool                          `json:"on_shift"`
	CoversWindow  bool                          `json:"covers_window"`
	ShiftWindows  []string                      `json:"shift_windows"`
	AbsenceReason *string                       `json:"absence_reason,omitempty"`
	Assignments   []StaffPoolAssignmentResponse `json:"assignments"`
}

// StaffPoolResponse is the 200 body of GET /instances/{id}/staff-pool.
type StaffPoolResponse struct {
	InstanceID      int64                    `json:"instance_id"`
	Title           string                   `json:"title"`
	Date            string                   `json:"date"`
	StartTime       string                   `json:"start_time"`
	EndTime         string                   `json:"end_time"`
	DienstplanInUse bool                     `json:"dienstplan_in_use"`
	Entries         []StaffPoolEntryResponse `json:"entries"`
}

// getStaffPool handles GET /api/timetable/instances/{id}/staff-pool.
func (rs *Resource) getStaffPool(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	pool, err := rs.TimetableData.GetStaffPoolForInstance(r.Context(), id)
	if err != nil {
		if base.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("instance not found")))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load staff pool failed", err))
		return
	}

	common.Respond(w, r, http.StatusOK, staffPoolResponseOf(pool), "Staff pool retrieved")
}

// staffPoolResponseOf maps the neutral service result onto the wire shape.
func staffPoolResponseOf(pool *scheduleSvc.StaffPoolResult) StaffPoolResponse {
	entries := make([]StaffPoolEntryResponse, 0, len(pool.Entries))
	for _, entry := range pool.Entries {
		assignments := make([]StaffPoolAssignmentResponse, 0, len(entry.Assignments))
		for _, assignment := range entry.Assignments {
			assignments = append(assignments, StaffPoolAssignmentResponse(assignment))
		}
		entries = append(entries, StaffPoolEntryResponse{
			StaffID:       entry.StaffID,
			DisplayName:   entry.DisplayName,
			Category:      entry.Category,
			OnShift:       entry.OnShift,
			CoversWindow:  entry.CoversWindow,
			ShiftWindows:  entry.ShiftWindows,
			AbsenceReason: entry.AbsenceReason,
			Assignments:   assignments,
		})
	}
	return StaffPoolResponse{
		InstanceID:      pool.Instance.ID,
		Title:           pool.Instance.Title,
		Date:            pool.Instance.Date.String(),
		StartTime:       timezone.WallClock(pool.Instance.StartTime).Format("15:04"),
		EndTime:         timezone.WallClock(pool.Instance.EndTime).Format("15:04"),
		DienstplanInUse: pool.DienstplanInUse,
		Entries:         entries,
	}
}
