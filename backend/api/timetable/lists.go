// Package timetable — issue #1565: lists from timetable slots.
//
//	POST /api/timetable/lists/options  → available list types for one date
//	POST /api/timetable/lists/preview  → JSON rows + counters for the UI
//	POST /api/timetable/lists/export   → rendered PDF/XLSX file
//
// Both endpoints share one request shape (the export adds a format field) and
// delegate entirely to services/slotlists. They are read-only derivations over
// selected schedule.activity_instances / schedule.instance_students (Plan) and
// active.visits / active.attendance (Ist); the existing emergency snapshot
// stays a separate, unfiltered Ist-list.
package timetable

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/services/slotlists"
)

type slotListRequest struct {
	Date         string   `json:"date"`
	Target       string   `json:"target"`
	PickupCohort string   `json:"pickup_cohort,omitempty"`
	Source       string   `json:"source"`
	Format       string   `json:"format,omitempty"`
	InstanceIDs  []int64  `json:"instance_ids,omitempty"`
	GroupIDs     []int64  `json:"group_ids,omitempty"`
	Classes      []string `json:"classes,omitempty"`
	GroupBy      string   `json:"group_by,omitempty"`
}

type slotListOptionsRequest struct {
	Date string `json:"date"`
}

// parseSlotListParams validates the shared request fields. Returns the
// params or writes an error response and returns false.
func (rs *Resource) parseSlotListParams(w http.ResponseWriter, r *http.Request, req slotListRequest) (slotlists.Params, bool) {
	date, err := berlinDate(req.Date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date must be YYYY-MM-DD")))
		return slotlists.Params{}, false
	}
	target := slotlists.Target(req.Target)
	if !target.Valid() {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("unknown target %q", req.Target)))
		return slotlists.Params{}, false
	}
	source := slotlists.Source(req.Source)
	if !source.Valid() {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("unknown source %q", req.Source)))
		return slotlists.Params{}, false
	}
	pickupCohort := slotlists.PickupCohort(req.PickupCohort)
	if target == slotlists.TargetPickupCohort && !pickupCohort.Valid() {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("unknown pickup_cohort %q", req.PickupCohort)))
		return slotlists.Params{}, false
	}
	groupBy := slotlists.GroupBy(req.GroupBy)
	if !groupBy.ValidFor(target) {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("grouping %q is not valid for target %q", req.GroupBy, req.Target)))
		return slotlists.Params{}, false
	}
	return slotlists.Params{
		Date:           date,
		Target:         target,
		PickupCohort:   pickupCohort,
		Source:         source,
		InstanceIDs:    req.InstanceIDs,
		InstanceIDsSet: req.InstanceIDs != nil,
		GroupIDs:       req.GroupIDs,
		Classes:        req.Classes,
		GroupBy:        groupBy,
	}, true
}

// listSlotListOptions handles POST /api/timetable/lists/options.
func (rs *Resource) listSlotListOptions(w http.ResponseWriter, r *http.Request) {
	if rs.slotListsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("slot lists service not wired")))
		return
	}

	var req slotListOptionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
		return
	}
	date, err := berlinDate(req.Date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date must be YYYY-MM-DD")))
		return
	}
	result, err := rs.slotListsService.ListOptions(r.Context(), date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("build slot list options failed", err))
		return
	}
	render.JSON(w, r, result)
}

// previewSlotList handles POST /api/timetable/lists/preview.
func (rs *Resource) previewSlotList(w http.ResponseWriter, r *http.Request) {
	if rs.slotListsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("slot lists service not wired")))
		return
	}

	var req slotListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
		return
	}
	params, ok := rs.parseSlotListParams(w, r, req)
	if !ok {
		return
	}

	result, err := rs.slotListsService.BuildList(r.Context(), params)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("build slot list failed", err))
		return
	}
	render.JSON(w, r, result)
}

// exportSlotList handles POST /api/timetable/lists/export.
func (rs *Resource) exportSlotList(w http.ResponseWriter, r *http.Request) {
	if rs.slotListsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("slot lists service not wired")))
		return
	}

	var req slotListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
		return
	}
	params, ok := rs.parseSlotListParams(w, r, req)
	if !ok {
		return
	}

	format := listexport.Format(req.Format)
	if format == "" {
		format = listexport.FormatPDF
	}
	if format != listexport.FormatPDF && format != listexport.FormatXLSX {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("unsupported format %q", req.Format)))
		return
	}

	file, err := rs.slotListsService.RenderList(r.Context(), params, format)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("render slot list failed", err))
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}
