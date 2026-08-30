// Package timetable — WP-B13 exception-conflict warnings.
//
//	GET /api/timetable/exception-conflicts?date=YYYY-MM-DD&date_to=YYYY-MM-DD
//
// Surfaces two classes of planning conflicts between activity_exceptions and
// student arrival expectations:
//
//   - cancelled_instance_with_scheduled_arrivals: a student is still flagged
//     as expected on an instance whose exception cancels the occurrence.
//   - modified_instance_time_mismatch: a modified exception pushes the
//     start time earlier than the student's resolved arrival, meaning the
//     student will miss the opening.
//
// Permission: SchedulesRead. Max range 14 days. Only today/future (same
// Berlin-local rule as /gaps). Empty list on no-conflict → 200, never 404.
// Detection itself lives in the TimetableData facade — the handler only parses
// the range, maps the result to the wire shape, and sorts.
package timetable

import (
	"errors"
	"log/slog"
	"net/http"
	"sort"

	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// Conflict kinds — aliased to the schedule service so the detection logic and
// the wire strings stay in lockstep.
const (
	ConflictKindCancelledArrivals = scheduleSvc.ConflictKindCancelledArrivals
	ConflictKindModifiedMismatch  = scheduleSvc.ConflictKindModifiedMismatch
)

// ConflictEntry is a single row in the conflict response. The optional string
// fields are populated per kind (reason only for cancelled, the two start-time
// strings only for modified) and serialise as omitempty.
type ConflictEntry struct {
	Kind               string `json:"kind"`
	Date               string `json:"date"`
	ActivityGroupID    int64  `json:"activity_group_id"`
	InstanceID         int64  `json:"instance_id"`
	ActivityTitle      string `json:"activity_title"`
	StudentID          int64  `json:"student_id"`
	ExpectedArrival    string `json:"expected_arrival,omitempty"`
	ArrivalSource      string `json:"arrival_source"`
	CancellationReason string `json:"cancellation_reason,omitempty"`
	OriginalStartTime  string `json:"original_start_time,omitempty"`
	ModifiedStartTime  string `json:"modified_start_time,omitempty"`
}

// ConflictsResponse is the 200 body for GET /exception-conflicts.
type ConflictsResponse struct {
	From      string          `json:"from"`
	To        string          `json:"to"`
	Conflicts []ConflictEntry `json:"conflicts"`
}

// getExceptionConflicts handles GET /api/timetable/exception-conflicts.
func (rs *Resource) getExceptionConflicts(w http.ResponseWriter, r *http.Request) {
	from, to, ok := rs.parseTodayFutureDateRange(w, r)
	if !ok {
		return
	}

	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}

	detected, err := rs.TimetableData.DetectExceptionConflicts(r.Context(), from, to, rs.getLogger())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("detect exception conflicts failed", err))
		return
	}

	conflicts := make([]ConflictEntry, 0, len(detected))
	for _, c := range detected {
		conflicts = append(conflicts, mapConflictEntry(c))
	}

	sort.SliceStable(conflicts, func(i, j int) bool {
		if conflicts[i].Date != conflicts[j].Date {
			return conflicts[i].Date < conflicts[j].Date
		}
		if conflicts[i].ActivityGroupID != conflicts[j].ActivityGroupID {
			return conflicts[i].ActivityGroupID < conflicts[j].ActivityGroupID
		}
		return conflicts[i].StudentID < conflicts[j].StudentID
	})

	rs.getLogger().Info("exception conflicts",
		slog.String("from", from.String()),
		slog.String("to", to.String()),
		slog.Int("conflict_count", len(conflicts)),
	)

	common.Respond(w, r, http.StatusOK, ConflictsResponse{
		From:      from.String(),
		To:        to.String(),
		Conflicts: conflicts,
	}, "Exception conflicts retrieved")
}

// mapConflictEntry lifts a detected conflict into the wire shape.
func mapConflictEntry(c scheduleSvc.ExceptionConflict) ConflictEntry {
	return ConflictEntry{
		Kind:               c.Kind,
		Date:               c.Date,
		ActivityGroupID:    c.ActivityGroupID,
		InstanceID:         c.InstanceID,
		ActivityTitle:      c.ActivityTitle,
		StudentID:          c.StudentID,
		ExpectedArrival:    c.ExpectedArrival,
		ArrivalSource:      c.ArrivalSource,
		CancellationReason: c.CancellationReason,
		OriginalStartTime:  c.OriginalStartTime,
		ModifiedStartTime:  c.ModifiedStartTime,
	}
}
