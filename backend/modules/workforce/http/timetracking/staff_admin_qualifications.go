package timetracking

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Nachweise (#4808): the kinds of proof of the school, the list over all
// staff and each person's own entries. Kinds and list are staff:stammdaten;
// a person reads their own entries without it.

// QualificationTypeResponse is one kind of proof on the wire.
type QualificationTypeResponse struct {
	ID                string `json:"id"`
	StandardKey       string `json:"standard_key,omitempty"`
	Name              string `json:"name"`
	ValidityMonths    *int   `json:"validity_months"`
	ReminderLeadWeeks int    `json:"reminder_lead_weeks"`
	Mandatory         bool   `json:"mandatory"`
	Archived          bool   `json:"archived"`
}

// QualificationTypeRequest binds the create and update of a kind. A missing
// lead takes the agreed eight weeks.
type QualificationTypeRequest struct {
	Name              string `json:"name"`
	ValidityMonths    *int   `json:"validity_months"`
	ReminderLeadWeeks *int   `json:"reminder_lead_weeks"`
	Mandatory         bool   `json:"mandatory"`
	Archived          bool   `json:"archived"`
}

// Bind reports a blank name at its field.
func (r *QualificationTypeRequest) Bind(_ *http.Request) error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("name is required")
	}
	return nil
}

func (r *QualificationTypeRequest) input() workforce.QualificationTypeInput {
	lead := workforce.DefaultQualificationReminderLeadWeeks
	if r.ReminderLeadWeeks != nil {
		lead = *r.ReminderLeadWeeks
	}
	return workforce.QualificationTypeInput{
		Name: r.Name, ValidityMonths: r.ValidityMonths, ReminderLeadWeeks: lead,
		Mandatory: r.Mandatory, Archived: r.Archived,
	}
}

// QualificationEntryResponse is one qualification with its status; a
// missing mandatory kind has no qualification_id.
type QualificationEntryResponse struct {
	StaffID         string         `json:"staff_id"`
	QualificationID *string        `json:"qualification_id"`
	TypeID          *string        `json:"type_id"`
	Name            string         `json:"name"`
	AcquiredOn      *calendar.Date `json:"acquired_on"`
	ExpiresOn       *calendar.Date `json:"expires_on"`
	Status          string         `json:"status"`
	Mandatory       bool           `json:"mandatory"`
}

// QualificationOverviewResponse is one line of the Nachweise list.
type QualificationOverviewResponse struct {
	QualificationEntryResponse
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (rs *StaffAdminResource) registerQualificationRoutes(r chi.Router, withTx common.Middleware) {
	stammdaten := common.RequiresPermission(permissions.StaffStammdaten)
	qualificationRead := common.RequiresAnyPermission(permissions.StaffStammdaten, permissions.TimeTrackingManage)
	r.With(stammdaten, withTx).Get("/qualification-types", rs.listQualificationTypes)
	r.With(stammdaten, withTx).Post("/qualification-types", rs.createQualificationType)
	r.With(stammdaten, withTx).Put("/qualification-types/{typeId}", rs.updateQualificationType)
	r.With(stammdaten, withTx).Get("/qualifications", rs.listQualificationOverview)
	r.With(qualificationRead, withTx).Get("/{id}/qualifications", rs.getStaffQualifications)
	// Every signed-in staff member reads their own Nachweise; the handler
	// resolves the caller and never takes a staff id from the request.
	r.With(withTx).Get("/me/qualifications", rs.getOwnQualifications)
}

func (rs *StaffAdminResource) listQualificationTypes(w http.ResponseWriter, r *http.Request) {
	types, err := rs.qualificationTypes.ListQualificationTypes(r.Context())
	if err != nil {
		renderQualificationTypeError(w, r, err)
		return
	}
	resp := make([]QualificationTypeResponse, 0, len(types))
	for _, kind := range types {
		resp = append(resp, toQualificationTypeResponse(kind))
	}
	common.Respond(w, r, http.StatusOK, resp, "Qualification types retrieved successfully")
}

func (rs *StaffAdminResource) createQualificationType(w http.ResponseWriter, r *http.Request) {
	req := &QualificationTypeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeWorkforceStammdatenInvalid))
		return
	}
	created, err := rs.qualificationTypes.CreateQualificationType(r.Context(), req.input())
	if err != nil {
		renderQualificationTypeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toQualificationTypeResponse(created), "Qualification type created successfully")
}

func (rs *StaffAdminResource) updateQualificationType(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "typeId", "invalid qualification type ID")
	if !ok {
		return
	}
	req := &QualificationTypeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeWorkforceStammdatenInvalid))
		return
	}
	updated, err := rs.qualificationTypes.UpdateQualificationType(r.Context(), id, req.input())
	if err != nil {
		renderQualificationTypeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toQualificationTypeResponse(updated), "Qualification type updated successfully")
}

func (rs *StaffAdminResource) listQualificationOverview(w http.ResponseWriter, r *http.Request) {
	rows, err := rs.qualifications.QualificationOverview(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	resp := make([]QualificationOverviewResponse, 0, len(rows))
	for _, row := range rows {
		resp = append(resp, QualificationOverviewResponse{
			QualificationEntryResponse: toQualificationEntryResponse(row.QualificationEntry),
			FirstName:                  row.FirstName, LastName: row.LastName,
		})
	}
	common.Respond(w, r, http.StatusOK, resp, "Qualifications retrieved successfully")
}

func (rs *StaffAdminResource) getStaffQualifications(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	rs.respondQualificationEntries(w, r, id)
}

// getOwnQualifications answers the caller's own Nachweise. An account
// without a staff record has none.
func (rs *StaffAdminResource) getOwnQualifications(w http.ResponseWriter, r *http.Request) {
	accountID := rs.identity(r.Context()).AccountID
	if accountID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token: no account id")))
		return
	}
	staffID, err := rs.PersonService.ResolveStaffIDByAccountID(r.Context(), accountID)
	if err != nil {
		if common.IsNotFound(err) {
			common.Respond(w, r, http.StatusOK, []QualificationEntryResponse{}, "Qualifications retrieved successfully")
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	rs.respondQualificationEntries(w, r, staffID)
}

func (rs *StaffAdminResource) respondQualificationEntries(w http.ResponseWriter, r *http.Request, staffID int64) {
	entries, err := rs.qualifications.StaffQualificationStatus(r.Context(), staffID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	resp := make([]QualificationEntryResponse, 0, len(entries))
	for _, entry := range entries {
		resp = append(resp, toQualificationEntryResponse(entry))
	}
	common.Respond(w, r, http.StatusOK, resp, "Qualifications retrieved successfully")
}

func renderQualificationTypeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, workforce.ErrQualificationTypeNameTaken):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeWorkforceQualificationTypeNameTaken))
	case errors.Is(err, workforce.ErrQualificationTypeNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeWorkforceQualificationTypeNotFound))
	case errors.Is(err, workforce.ErrInvalidStaffRecord):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeWorkforceStammdatenInvalid))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

func toQualificationTypeResponse(kind workforce.QualificationType) QualificationTypeResponse {
	return QualificationTypeResponse{
		ID: decimalID(kind.ID), StandardKey: kind.StandardKey, Name: kind.Name, ValidityMonths: kind.ValidityMonths,
		ReminderLeadWeeks: kind.ReminderLeadWeeks, Mandatory: kind.Mandatory, Archived: kind.Archived,
	}
}

func toQualificationEntryResponse(entry workforce.QualificationEntry) QualificationEntryResponse {
	return QualificationEntryResponse{
		StaffID: decimalID(entry.StaffID), QualificationID: optionalDecimalID(entry.QualificationID), TypeID: optionalDecimalID(entry.TypeID), Name: entry.Name,
		AcquiredOn: optionalDate(entry.AcquiredOn), ExpiresOn: optionalDate(entry.ExpiresOn),
		Status: entry.Status, Mandatory: entry.Mandatory,
	}
}
