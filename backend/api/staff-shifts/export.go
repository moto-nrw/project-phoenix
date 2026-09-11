package staffshifts

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
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
	var req planExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.invalid(w, r, errors.New("invalid JSON body"))
		return
	}

	file, err := rs.planning.ExportPlan(r.Context(), workforce.PlanExportRequest{
		From:          req.From,
		To:            req.To,
		Template:      req.Template,
		Variant:       req.Variant,
		Format:        req.Format,
		AllowInternal: rs.runtime.CanExportInternalPlan(r.Context()),
	})
	if err != nil {
		if kind := classify(err); kind != FailureInternal {
			rs.runtime.Failure(w, r, kind, err)
			return
		}
		rs.runtime.Failure(w, r, FailureInternal, &ClientMessageError{Message: "render dienstplan export failed", Cause: err})
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
