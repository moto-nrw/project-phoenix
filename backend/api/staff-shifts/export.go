package staffshifts

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/services/planexport"
)

// planExportRequest is the shared body of both plan export routes (#2079).
// Dates are "YYYY-MM-DD"; the service widens them to whole weeks.
type planExportRequest struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Template string `json:"template"`
	Variant  string `json:"variant"`
	Format   string `json:"format"`
}

// exportPlan handles POST /api/staff-shifts/export — the printable
// Dienstplan week. It reads the same overview projection the screen does, so
// the permission set matches GET /overview exactly.
func (rs *Resource) exportPlan(w http.ResponseWriter, r *http.Request) {
	if rs.PlanExport == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("plan export service not wired")))
		return
	}

	var req planExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
		return
	}

	params, err := planexport.ParseParams(req.From, req.To, req.Template, req.Variant, req.Format)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	file, err := rs.PlanExport.ExportDienstplan(r.Context(), params)
	if err != nil {
		if errors.Is(err, planexport.ErrInvalidParams) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("render dienstplan export failed", err))
		return
	}

	writePlanExportFile(w, file.ContentType, file.Filename, file.Data)
}

// writePlanExportFile streams a rendered document as a download, matching
// how every other export route in this codebase answers.
func writePlanExportFile(w http.ResponseWriter, contentType, filename string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
