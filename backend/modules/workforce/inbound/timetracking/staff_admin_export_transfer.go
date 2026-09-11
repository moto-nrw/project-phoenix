package timetracking

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// Manual SFTP transfer of the cross-staff time export (#3050).
//
// The handler builds the file with the SAME Export() call the download uses
// and hands the resulting bytes to the Export Transfer capability. That is why
// the transferred file cannot drift from the downloaded one: there is one
// file, produced once, either written to the response or sent onwards.

// transferExportRequest is the POST body. The parameters travel in the body,
// not the query: a POST proxy forwards the body, and relying on a forwarded
// query string is how the first version silently lost every parameter.
type transferExportRequest struct {
	Year        int    `json:"year"`
	Month       int    `json:"month"`
	Granularity string `json:"granularity"`
	Format      string `json:"format"`
	TimeFormat  string `json:"time_format"`
}

// parseTimeExportBody reads the export selection from the request body.
// Granularity, format and time format default in the service, so their
// canonical values stay in one place.
func parseTimeExportBody(r *http.Request) (workforce.TimeExportRequest, error) {
	var body transferExportRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return workforce.TimeExportRequest{}, errors.New("request body must be JSON")
	}
	if body.Year == 0 {
		return workforce.TimeExportRequest{}, errors.New("year is required")
	}
	return workforce.TimeExportRequest{
		Year:        body.Year,
		Month:       body.Month,
		Granularity: body.Granularity,
		Format:      body.Format,
		TimeFormat:  body.TimeFormat,
	}, nil
}

// exportSFTPStatus handles GET /api/staff/time-tracking/export/sftp-status —
// what the export dialog needs to decide whether it may offer the transfer at
// all, and to name the destination. Credential-free by construction: the
// status type has no password field.
func (rs *StaffAdminResource) exportSFTPStatus(w http.ResponseWriter, r *http.Request) {
	if rs.ExportTransfer == nil {
		common.Respond(w, r, http.StatusOK, ExportTransferStatus{}, "SFTP status")
		return
	}
	status, err := rs.ExportTransfer.Status(r.Context())
	if err != nil {
		rs.logger.Error("failed to resolve export transfer status", "error", err.Error())
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, status, "SFTP status")
}

// transferExportViaSFTP handles POST /api/staff/time-tracking/export/sftp.
// Same permission as the export itself; the export parameters come from the
// query, and the remote path never does.
//
// A failed transfer answers 200 with success=false, not 5xx. That is not
// cosmetic: TenantTxMiddleware rolls the tenant transaction back on a 5xx,
// which would delete the journal row recording the failed attempt — the exact
// row the audit requirement asks for. 5xx stays reserved for failures where
// nothing was attempted or nothing could be recorded.
func (rs *StaffAdminResource) transferExportViaSFTP(w http.ResponseWriter, r *http.Request) {
	req, err := parseTimeExportBody(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims := rs.identity(r.Context())
	if rs.ExportTransfer == nil {
		common.Respond(w, r, http.StatusOK, ExportTransferOutcome{Reason: exportTransferReasonNotConfigured}, "SFTP transfer")
		return
	}

	// The status is checked BEFORE the export is built. Reading the whole
	// staff's working time — and writing a GDPR access-audit row for it — to
	// then discover there is nowhere to send it would be a disclosure for
	// nothing.
	status, err := rs.ExportTransfer.Status(r.Context())
	if err != nil {
		rs.logger.Error("failed to resolve export transfer status", "error", err.Error())
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	if !status.Ready {
		common.Respond(w, r, http.StatusOK, ExportTransferOutcome{
			Reason: exportTransferReasonNotConfigured,
		}, "SFTP transfer")
		return
	}

	file, err := rs.TimeExportService.ExportStaffTime(r.Context(), req, claims.AccountID, strings.Join(claims.Roles, ","))
	if err != nil {
		rs.logger.Error("failed to build time tracking export for transfer", "error", err.Error())
		common.RenderError(w, r, common.RenderWithRules(err, timeExportErrorRules, common.ErrorInternalServer))
		return
	}

	actorName := strings.TrimSpace(claims.DisplayName)

	outcome, err := rs.ExportTransfer.Transfer(r.Context(), ExportTransferRequest{
		Kind:           exportTransferKindStaffTimeTracking,
		Format:         req.Format,
		Filename:       file.Filename,
		Data:           file.Data,
		ActorAccountID: claims.AccountID,
		ActorName:      actorName,
	})
	if err != nil {
		rs.logger.Error("failed to transfer time tracking export", "error", err.Error())
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, outcome, "SFTP transfer")
}
