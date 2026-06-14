package operator

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

type resolveUnregisteredTagScanRequest struct {
	Note string `json:"note,omitempty"`
}

func (req *resolveUnregisteredTagScanRequest) Bind(_ *http.Request) error {
	req.Note = strings.TrimSpace(req.Note)
	return nil
}

func (rs *Resource) ListUnregisteredTagScans(w http.ResponseWriter, r *http.Request) {
	filter, err := parseUnregisteredTagScanFilter(r)
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	scans, err := rs.unregisteredTagScans.ListForOperator(r.Context(), filter)
	if err != nil {
		common.RenderError(w, r, ErrInternal("Failed to list unregistered RFID scans"))
		return
	}
	common.Respond(w, r, http.StatusOK, scans, "Unregistered RFID scans retrieved successfully")
}

func (rs *Resource) ResolveUnregisteredTagScan(w http.ResponseWriter, r *http.Request) {
	scanID, err := parseInt64Param(chi.URLParam(r, "id"), "invalid scan ID")
	if err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	req := &resolveUnregisteredTagScanRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	operatorID := int64(claims.ID)
	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	scan, err := rs.unregisteredTagScans.Resolve(r.Context(), scanID, operatorID, note)
	if err != nil {
		var dbErr *modelBase.DatabaseError
		if errors.As(err, &dbErr) {
			common.RenderError(w, r, ErrInternal("Failed to resolve unregistered RFID scan"))
			return
		}
		common.RenderError(w, r, ErrInvalidRequest(err))
		return
	}
	common.Respond(w, r, http.StatusOK, scan, "Unregistered RFID scan resolved successfully")
}

func parseUnregisteredTagScanFilter(r *http.Request) (auditModels.UnregisteredTagScanFilter, error) {
	query := r.URL.Query()
	filter := auditModels.UnregisteredTagScanFilter{
		UnresolvedOnly: query.Get("resolved") != "all",
	}
	if schoolID := strings.TrimSpace(query.Get("school_id")); schoolID != "" {
		id, err := parseInt64Param(schoolID, "invalid school ID")
		if err != nil {
			return filter, err
		}
		filter.SchoolID = &id
	}
	if orgID := strings.TrimSpace(query.Get("organization_id")); orgID != "" {
		id, err := parseInt64Param(orgID, "invalid organization ID")
		if err != nil {
			return filter, err
		}
		filter.OrganizationID = &id
	}
	return filter, nil
}

func parseInt64Param(value, message string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New(message)
	}
	return id, nil
}
