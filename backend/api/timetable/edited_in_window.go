// Package timetable — edited-occurrence probe (#1875).
//
//	GET /api/timetable/instances/edited-in-window
//	    ?activity_group_id=ID&from=YYYY-MM-DD&to=YYYY-MM-DD
//
// Returns the planned, template-backed occurrences of one template in the
// window whose content diverges from the template — the single-occurrence
// edits ("Nur diesen Termin") that a series re-plan ("Alle Termine" /
// "Dieser und alle folgenden") would silently discard. The admin planner calls
// this before a series edit so it can warn and list the affected dates.
// Read-only, permission: SchedulesRead.
package timetable

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// maxEditedWindowRangeDays caps the probe window. Unlike the weekly list
// endpoint (56 days), a series edit spans today → calendar-period end (up to a
// school year), so the planner probes that whole future window in one call.
// The scan is one template's instances — ~50 rows even across a year — so a
// generous cap stays cheap while covering any real period.
const maxEditedWindowRangeDays = 400

// editedOccurrenceItem is one individually-adjusted occurrence.
type editedOccurrenceItem struct {
	InstanceID int64  `json:"instance_id"`
	Date       string `json:"date"`
	StartTime  string `json:"start_time"`
	Title      string `json:"title"`
	// Changes lists the diverging field categories ("title", "notes", "room",
	// "time", "staff", "students") — stable machine-readable strings the
	// frontend maps to German labels.
	Changes []string `json:"changes"`
}

// editedInWindowResponse carries the total count plus the concrete occurrences
// so the frontend can both warn ("X Termine …") and list the dates a user may
// want to re-edit afterwards.
type editedInWindowResponse struct {
	Count       int                    `json:"count"`
	Occurrences []editedOccurrenceItem `json:"occurrences"`
}

// editedInWindow handles GET /instances/edited-in-window.
func (rs *Resource) editedInWindow(w http.ResponseWriter, r *http.Request) {
	if rs.MaterializationService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("materialization service not wired")))
		return
	}

	q := r.URL.Query()
	groupStr := q.Get("activity_group_id")
	fromStr := q.Get("from")
	toStr := q.Get("to")
	if groupStr == "" || fromStr == "" || toStr == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("activity_group_id, from and to query params are required")))
		return
	}

	activityGroupID, err := strconv.ParseInt(groupStr, 10, 64)
	if err != nil || activityGroupID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid activity_group_id")))
		return
	}

	from, err := berlinDate(fromStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid from format, expected YYYY-MM-DD")))
		return
	}
	to, err := berlinDate(toStr)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("invalid to format, expected YYYY-MM-DD")))
		return
	}
	if to.Before(from) {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("'to' must be on or after 'from'")))
		return
	}
	if inclusiveDayCount(from, to) > maxEditedWindowRangeDays {
		common.RenderError(w, r, common.ErrorInvalidRequest(
			fmt.Errorf("date range exceeds maximum of %d days", maxEditedWindowRangeDays)))
		return
	}

	occurrences, err := rs.MaterializationService.DetectEditedInWindow(r.Context(), activityGroupID, from, to)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("detect edited occurrences failed", err))
		return
	}

	resp := editedInWindowResponse{
		Count:       len(occurrences),
		Occurrences: toEditedItems(occurrences),
	}
	common.Respond(w, r, http.StatusOK, resp, "Edited occurrences retrieved")
}

// toEditedItems maps the service result to the wire shape (never nil so the
// JSON array is always present).
func toEditedItems(occurrences []scheduleSvc.EditedOccurrence) []editedOccurrenceItem {
	items := make([]editedOccurrenceItem, 0, len(occurrences))
	for _, o := range occurrences {
		items = append(items, editedOccurrenceItem{
			InstanceID: o.InstanceID,
			Date:       o.Date.Format(dateLayout),
			StartTime:  o.StartTime,
			Title:      o.Title,
			Changes:    o.Changes,
		})
	}
	return items
}
