// Package timetable — GET /api/timetable/offering-sources handler (#2137).
//
// Editor support for offering-sourced Regeltermine: lists the tenant's
// Betreuungsangebote (optionally restricted to those fitting a calendar
// period) with per-Jahrgang counts of approved children (live filter
// preview / empty-filter warning) and the templates already sourcing each
// offering (overlap Hinweis before saving a second Termin over the same
// children).
package timetable

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

type offeringSourcesResponse struct {
	Offerings []offeringSourceOptionResponse `json:"offerings"`
}

// offeringSourceOptionResponse mirrors enrollment.OfferingSourceOption with
// string-keyed grade counts (JSON objects cannot carry int keys).
type offeringSourceOptionResponse struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	PhaseID           int64  `json:"phase_id"`
	PhaseName         string `json:"phase_name"`
	PhaseServiceStart string `json:"phase_service_start,omitempty"`
	TotalCount        int    `json:"total_count"`
	// GradeCounts maps Jahrgang (as string) → approved children; key "0"
	// collects children without a derivable grade.
	GradeCounts            map[string]int                    `json:"grade_counts"`
	SourcedTemplates       []offeringSourcedTemplateResponse `json:"sourced_templates"`
	LegacyLinkedTemplateID *int64                            `json:"legacy_linked_template_id,omitempty"`
}

type offeringSourcedTemplateResponse struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	GradeLevels []int  `json:"grade_levels,omitempty"`
}

// listOfferingSources handles GET /api/timetable/offering-sources.
func (rs *Resource) listOfferingSources(w http.ResponseWriter, r *http.Request) {
	if rs.OfferingSourceOptions == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}
	var calendarPeriodID *int64
	if raw := r.URL.Query().Get("calendar_period_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("calendar_period_id must be a positive integer")))
			return
		}
		calendarPeriodID = &id
	}
	options, err := rs.OfferingSourceOptions.ListOfferingSourceOptions(r.Context(), calendarPeriodID)
	if err != nil {
		if errors.Is(err, scheduleSvc.ErrOfferingSourceInvalid) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("list offering sources failed", err))
		return
	}
	resp := offeringSourcesResponse{Offerings: make([]offeringSourceOptionResponse, 0, len(options))}
	for _, option := range options {
		gradeCounts := make(map[string]int, len(option.GradeCounts))
		for grade, count := range option.GradeCounts {
			gradeCounts[strconv.Itoa(grade)] = count
		}
		sourced := make([]offeringSourcedTemplateResponse, 0, len(option.SourcedTemplates))
		for _, tmpl := range option.SourcedTemplates {
			sourced = append(sourced, offeringSourcedTemplateResponse{
				ID:          tmpl.ID,
				Name:        tmpl.Name,
				GradeLevels: tmpl.GradeLevels,
			})
		}
		resp.Offerings = append(resp.Offerings, offeringSourceOptionResponse{
			ID:                     option.ID,
			Name:                   option.Name,
			PhaseID:                option.PhaseID,
			PhaseName:              option.PhaseName,
			PhaseServiceStart:      option.PhaseServiceStart,
			TotalCount:             option.TotalCount,
			GradeCounts:            gradeCounts,
			SourcedTemplates:       sourced,
			LegacyLinkedTemplateID: option.LegacyLinkedTemplateID,
		})
	}
	common.Respond(w, r, http.StatusOK, resp, "Offering sources retrieved")
}

// combinedOfferingCountsResponse mirrors
// enrollment.OfferingSourceCombinedCounts with string-keyed grade counts.
type combinedOfferingCountsResponse struct {
	TotalCount  int            `json:"total_count"`
	GradeCounts map[string]int `json:"grade_counts"`
}

// getCombinedOfferingSourceCounts handles
// GET /api/timetable/offering-sources/combined-counts?ids=1,2,3[&calendar_period_id=...]
// (multi-source follow-up to #2137): the deduplicated child counts across a
// selection of offerings, so the editor previews the EXACT roster size the
// union of the selected Angebote would seed. Mixed-phase or invalid
// selections reject with 400, the same verdict a save would produce.
func (rs *Resource) getCombinedOfferingSourceCounts(w http.ResponseWriter, r *http.Request) {
	if rs.OfferingSourceOptions == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return
	}
	offeringIDs, err := parseOfferingIDList(r.URL.Query().Get("ids"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	var calendarPeriodID *int64
	if raw := r.URL.Query().Get("calendar_period_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("calendar_period_id must be a positive integer")))
			return
		}
		calendarPeriodID = &id
	}
	counts, err := rs.OfferingSourceOptions.CombinedOfferingSourceCounts(r.Context(), offeringIDs, calendarPeriodID)
	if err != nil {
		if errors.Is(err, scheduleSvc.ErrOfferingSourceInvalid) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("combined offering source counts failed", err))
		return
	}
	gradeCounts := make(map[string]int, len(counts.GradeCounts))
	for grade, count := range counts.GradeCounts {
		gradeCounts[strconv.Itoa(grade)] = count
	}
	common.Respond(w, r, http.StatusOK, combinedOfferingCountsResponse{
		TotalCount:  counts.TotalCount,
		GradeCounts: gradeCounts,
	}, "Combined offering source counts retrieved")
}

// parseOfferingIDList decodes the comma-separated ids query parameter into
// positive offering ids; empty segments are tolerated, an empty result is an
// error (the endpoint is meaningless without a selection). The count is
// capped before any lookup happens: the service resolves ids individually,
// so an unbounded list would translate into thousands of queries inside one
// tenant transaction.
func parseOfferingIDList(raw string) ([]int64, error) {
	segments := strings.Split(raw, ",")
	if len(segments) > scheduleSvc.MaxOfferingSourcesPerTemplate {
		return nil, fmt.Errorf("at most %d ids are supported", scheduleSvc.MaxOfferingSourcesPerTemplate)
	}
	offeringIDs := make([]int64, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		id, err := strconv.ParseInt(segment, 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.New("ids must be positive integers")
		}
		offeringIDs = append(offeringIDs, id)
	}
	if len(offeringIDs) == 0 {
		return nil, errors.New("ids is required")
	}
	return offeringIDs, nil
}
