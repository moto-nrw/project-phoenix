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
	ListKind     string   `json:"list_kind,omitempty"`
	InstanceIDs  []string `json:"instance_ids,omitempty"`
	GroupIDs     []string `json:"group_ids,omitempty"`
	Classes      []string `json:"classes,omitempty"`
	GroupBy      string   `json:"group_by,omitempty"`
}

type slotListOptionsRequest struct {
	Date string `json:"date"`
}

// parseSlotListParams validates the shared request fields. Returns the
// params or writes an error response and returns false.
func (rs *Resource) parseSlotListParams(w http.ResponseWriter, r *http.Request, req slotListRequest) (slotlists.Params, bool) {
	instanceIDs, err := parseSlotListIDList(req.InstanceIDs, "instance_ids")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return slotlists.Params{}, false
	}
	groupIDs, err := parseSlotListIDList(req.GroupIDs, "group_ids")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return slotlists.Params{}, false
	}
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
	listKind := slotlists.ListKind(req.ListKind)
	if target != slotlists.TargetSlots && listKind != slotlists.ListKindNone {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("list_kind is not valid for target %q", req.Target)))
		return slotlists.Params{}, false
	}
	if !listKind.Valid() {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("unknown list_kind %q", req.ListKind)))
		return slotlists.Params{}, false
	}
	return slotlists.Params{
		Date:           date,
		Target:         target,
		PickupCohort:   pickupCohort,
		Source:         source,
		ListKind:       listKind,
		InstanceIDs:    instanceIDs,
		InstanceIDsSet: req.InstanceIDs != nil,
		GroupIDs:       groupIDs,
		Classes:        req.Classes,
		GroupBy:        groupBy,
	}, true
}

func parseSlotListIDList(values []string, field string) ([]int64, error) {
	if values == nil {
		return nil, nil
	}
	ids := make([]int64, 0, len(values))
	for _, raw := range values {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%s must contain positive int64 strings", field)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// listSlotListOptions handles POST /api/timetable/lists/options.
func (rs *Resource) listSlotListOptions(w http.ResponseWriter, r *http.Request) {
	if rs.SlotListsService == nil {
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
	result, err := rs.SlotListsService.ListOptions(r.Context(), date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("build slot list options failed", err))
		return
	}
	render.JSON(w, r, result)
}

// previewSlotList handles POST /api/timetable/lists/preview.
func (rs *Resource) previewSlotList(w http.ResponseWriter, r *http.Request) {
	if rs.SlotListsService == nil {
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

	result, err := rs.SlotListsService.BuildList(r.Context(), params)
	if err != nil {
		if errors.Is(err, slotlists.ErrPickupCohortPastDate) ||
			errors.Is(err, slotlists.ErrReconciliationFutureDate) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("build slot list failed", err))
		return
	}
	render.JSON(w, r, result)
}

// exportSlotList handles POST /api/timetable/lists/export.
func (rs *Resource) exportSlotList(w http.ResponseWriter, r *http.Request) {
	if rs.SlotListsService == nil {
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

	file, err := rs.SlotListsService.RenderList(r.Context(), params, format)
	if err != nil {
		if errors.Is(err, slotlists.ErrPickupCohortPastDate) ||
			errors.Is(err, slotlists.ErrReconciliationFutureDate) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("render slot list failed", err))
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}
