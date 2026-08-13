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
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// editedOccurrenceItem is one individually-adjusted occurrence.
type editedOccurrenceItem struct {
	InstanceID int64  `json:"instance_id"`
	Date       string `json:"date"`
	StartTime  string `json:"start_time"`
	Title      string `json:"title"`
	// Changes lists the diverging field categories ("title", "notes", "room",
	// "time", "staff", "students", "attendance") — stable machine-readable
	// strings the frontend maps to German labels.
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
	// No upper cap on the window: a series re-plan spans today → calendar-period
	// end, and custom periods have no maximum duration. Capping here (returning
	// 400) would make the advisory frontend probe fail-open and re-plan without
	// a warning for long periods — silently discarding edits past the cap. The
	// scan is one template's instances via an indexed date-range query and is
	// bounded by the rows that actually exist, so a wide window stays cheap.

	// include_deletions=true additionally reports individually-deleted
	// occurrences (cancelled exceptions). The "following" split path sets it,
	// since a split resurrects those under the successor template; same-template
	// re-plans ("Alle Termine", direct Regeltermin edit) preserve them, so they
	// leave it off to avoid false warnings.
	includeDeletions := q.Get("include_deletions") == "true"

	occurrences, err := rs.MaterializationService.DetectEditedInWindow(r.Context(), activityGroupID, from, to, includeDeletions)
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
