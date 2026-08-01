// Package timetable — issue #2079: the printable Betreuungsplan week.
//
//	POST /api/timetable/betreuungsplan/export → rendered PDF/XLSX file
//
// Read-only projection of the same materialized instances the weekly planner
// shows, laid out as a matrix (one row per Angebot, Monday to Friday). The
// Dienstplan counterpart lives at POST /api/staff-shifts/export.
package timetable

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/planexport"
)

// betreuungsplanExportRequest mirrors the Dienstplan export body so both
// routes speak the same shape. Dates are "YYYY-MM-DD"; the service widens
// them to whole weeks.
type betreuungsplanExportRequest struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Template string `json:"template"`
	Variant  string `json:"variant"`
	Format   string `json:"format"`
}

// exportBetreuungsplan handles POST /api/timetable/betreuungsplan/export.
func (rs *Resource) exportBetreuungsplan(w http.ResponseWriter, r *http.Request) {
	if rs.PlanExportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("plan export service not wired")))
		return
	}

	var req betreuungsplanExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
		return
	}
	if req.Template == "" {
		// The care plan has exactly one row axis today; not making the client
		// spell it out keeps the request honest rather than ceremonial.
		req.Template = string(planexport.TemplateByOffering)
	}

	params, err := planexport.ParseParams(req.From, req.To, req.Template, req.Variant, req.Format)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if params.Variant == planexport.VariantInternal && !common.CanExportInternalPlan(r.Context()) {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("internal plan exports require schedules:manage")))
		return
	}

	file, err := rs.PlanExportService.ExportBetreuungsplan(r.Context(), params)
	if err != nil {
		if errors.Is(err, planexport.ErrInvalidParams) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("render betreuungsplan export failed", err))
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}
