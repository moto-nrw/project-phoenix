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
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

type offeringSourcesResponse struct {
	Offerings []offeringSourceOptionResponse `json:"offerings"`
}

// offeringSourceOptionResponse mirrors enrollment.OfferingSourceOption with
// string-keyed grade counts (JSON objects cannot carry int keys).
type offeringSourceOptionResponse struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	PhaseID    int64  `json:"phase_id"`
	PhaseName  string `json:"phase_name"`
	TotalCount int    `json:"total_count"`
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
			TotalCount:             option.TotalCount,
			GradeCounts:            gradeCounts,
			SourcedTemplates:       sourced,
			LegacyLinkedTemplateID: option.LegacyLinkedTemplateID,
		})
	}
	common.Respond(w, r, http.StatusOK, resp, "Offering sources retrieved")
}
