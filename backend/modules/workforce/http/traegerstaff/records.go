// Package traegerstaff serves the Personalstamm of ADR 0089 (#4705): the
// Träger-Portal's staff records at /traeger/staff-records and the release
// „Personal beim Träger“ in the Operator portal.
package traegerstaff

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
)

// RecordsResource serves the staff records of the session's Träger. Every
// route needs the Träger right „Personal“; the Träger comes from the
// session, never from the request, and the owner opens the transactions
// itself (ADR 0105).
type RecordsResource struct {
	records      workforce.TraegerStaffRecords
	requireStaff common.Middleware
}

// NewRecordsResource builds the resource; the root binds requireStaff to
// the Träger right traeger_staff:manage (ADR 0084 section 4).
func NewRecordsResource(records workforce.TraegerStaffRecords, requireStaff common.Middleware) *RecordsResource {
	if records == nil || requireStaff == nil {
		panic("workforce traeger staff: the records and the right check are required")
	}
	return &RecordsResource{records: records, requireStaff: requireStaff}
}

// TraegerRouter returns the routes mounted at /traeger/staff-records.
func (rs *RecordsResource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
		staff := r.With(rs.requireStaff)
		staff.Get("/", rs.overview)
		staff.Post("/", rs.link)
		staff.Post("/link-preview", rs.preview)
		staff.Get("/{id}", rs.record)
		staff.Delete("/{id}", rs.dissolve)
		staff.Put("/{id}/personnel-number", rs.personnelNumber)
		staff.Get("/{id}/financial", rs.financial)
		staff.Post("/{id}/financial/reveal", rs.revealFinancial)
		staff.Put("/{id}/financial", rs.updateFinancial)
	})
	return r
}

func actor(w http.ResponseWriter, r *http.Request) (workforce.TraegerStaffActor, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 || principal.AccountID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return workforce.TraegerStaffActor{}, false
	}
	return workforce.TraegerStaffActor{OrganizationID: principal.OrganizationID(), AccountID: principal.AccountID()}, true
}

func actorAndRecord(w http.ResponseWriter, r *http.Request) (workforce.TraegerStaffActor, int64, bool) {
	session, ok := actor(w, r)
	if !ok {
		return workforce.TraegerStaffActor{}, 0, false
	}
	id, ok := common.ParseInt64IDWithError(w, r, "id", "invalid staff record ID")
	return session, id, ok
}

func (rs *RecordsResource) overview(w http.ResponseWriter, r *http.Request) {
	session, ok := actor(w, r)
	if !ok {
		return
	}
	overview, err := rs.records.Overview(r.Context(), session)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, overview, "Staff records retrieved successfully")
}

func (rs *RecordsResource) record(w http.ResponseWriter, r *http.Request) {
	session, id, ok := actorAndRecord(w, r)
	if !ok {
		return
	}
	detail, err := rs.records.Record(r.Context(), session, id)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Staff record retrieved successfully")
}

// linkRequest names the employments to join and, per differing field, the
// chosen source: "record" or the ID of a school.
type linkRequest struct {
	StaffRecordID *int64                            `json:"staff_record_id"`
	Members       []workforce.TraegerStaffMemberRef `json:"members"`
	Choices       map[string]string                 `json:"choices"`
}

func (req *linkRequest) Bind(*http.Request) error {
	if len(req.Members) == 0 {
		return errors.New("members is required")
	}
	return nil
}

func (req *linkRequest) toInput() workforce.TraegerStaffLinkRequest {
	return workforce.TraegerStaffLinkRequest{RecordID: req.StaffRecordID, Members: req.Members, Choices: req.Choices}
}

func (rs *RecordsResource) preview(w http.ResponseWriter, r *http.Request) {
	session, ok := actor(w, r)
	if !ok {
		return
	}
	req := &linkRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeWorkforceTraegerStaffLinkInvalid, "members"))
		return
	}
	preview, err := rs.records.PreviewLink(r.Context(), session, req.toInput())
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, preview, "Staff record link previewed successfully")
}

func (rs *RecordsResource) link(w http.ResponseWriter, r *http.Request) {
	session, ok := actor(w, r)
	if !ok {
		return
	}
	req := &linkRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeWorkforceTraegerStaffLinkInvalid, "members"))
		return
	}
	detail, err := rs.records.Link(r.Context(), session, req.toInput())
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, detail, "Staff record linked successfully")
}

func (rs *RecordsResource) dissolve(w http.ResponseWriter, r *http.Request) {
	session, id, ok := actorAndRecord(w, r)
	if !ok {
		return
	}
	if err := rs.records.Dissolve(r.Context(), session, id); err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int64{"id": id}, "Staff record dissolved successfully")
}

// personnelNumberRequest sets or, with null, clears the Träger's number.
type personnelNumberRequest struct {
	PersonnelNumber json.RawMessage `json:"personnel_number"`

	value *string
}

func (req *personnelNumberRequest) Bind(*http.Request) error {
	if len(req.PersonnelNumber) == 0 {
		return errors.New("personnel_number is required")
	}
	if err := json.Unmarshal(req.PersonnelNumber, &req.value); err != nil {
		return errors.New("personnel_number must be a string or null")
	}
	return nil
}

func (rs *RecordsResource) personnelNumber(w http.ResponseWriter, r *http.Request) {
	session, id, ok := actorAndRecord(w, r)
	if !ok {
		return
	}
	req := &personnelNumberRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeWorkforcePersonnelNumberInvalid, "personnel_number"))
		return
	}
	detail, err := rs.records.UpdatePersonnelNumber(r.Context(), session, id, req.value)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Personnel number updated successfully")
}

func (rs *RecordsResource) financial(w http.ResponseWriter, r *http.Request) {
	rs.serveFinancial(w, r, false)
}

// revealFinancial is a POST: serving the values as entered writes an access
// entry at every Einsatzort.
func (rs *RecordsResource) revealFinancial(w http.ResponseWriter, r *http.Request) {
	rs.serveFinancial(w, r, true)
}

func (rs *RecordsResource) serveFinancial(w http.ResponseWriter, r *http.Request, reveal bool) {
	session, id, ok := actorAndRecord(w, r)
	if !ok {
		return
	}
	financial, err := rs.records.Financial(r.Context(), session, id, reveal)
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, financial, "Financial data retrieved successfully")
}

type financialRequest struct {
	IBAN                 *string `json:"iban"`
	TaxID                *string `json:"tax_id"`
	SocialSecurityNumber *string `json:"social_security_number"`
}

func (req *financialRequest) Bind(*http.Request) error { return nil }

func (rs *RecordsResource) updateFinancial(w http.ResponseWriter, r *http.Request) {
	session, id, ok := actorAndRecord(w, r)
	if !ok {
		return
	}
	req := &financialRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeWorkforceStammdatenInvalid))
		return
	}
	err := rs.records.UpdateFinancial(r.Context(), session, id, workforce.StammdatenFinancialInput{
		IBAN: req.IBAN, TaxID: req.TaxID, SocialSecurityNumber: req.SocialSecurityNumber,
	})
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int64{"id": id}, "Financial data updated successfully")
}

// renderError maps the Personalstamm's refusals onto their codes.
func renderError(w http.ResponseWriter, r *http.Request, err error) {
	if link, ok := errors.AsType[*workforce.TraegerStaffLinkError](err); ok {
		common.RenderError(w, r, common.ErrorInvalidOnField(err, common.CodeWorkforceTraegerStaffLinkInvalid, link.Field))
		return
	}
	common.RenderError(w, r, recordErrors(err))
}

var recordErrors = common.RulesRenderer([]common.ErrorRule{
	{Target: workforce.ErrTraegerStaffNotReleased, Render: coded(common.ErrorConflictWithCode, common.CodeWorkforceTraegerStaffNotReleased)},
	{Target: workforce.ErrTraegerStaffRecordNotFound, Render: coded(common.ErrorNotFoundWithCode, common.CodeWorkforceTraegerStaffRecordNotFound)},
	{Target: workforce.ErrTraegerStaffAlreadyLinked, Render: coded(common.ErrorConflictWithCode, common.CodeWorkforceTraegerStaffAlreadyLinked)},
	{Target: workforce.ErrPersonnelNumberTaken, Render: coded(common.ErrorConflictWithCode, common.CodeWorkforcePersonnelNumberTaken)},
	{Target: workforce.ErrPersonnelNumberInvalid, Render: onField(common.CodeWorkforcePersonnelNumberInvalid, "personnel_number")},
	{Target: workforce.ErrStaffStammdatenInvalid, Render: coded(common.ErrorInvalidRequestWithCode, common.CodeWorkforceStammdatenInvalid)},
}, common.ErrorInternalServer)

func coded(renderer func(error, string) render.Renderer, code string) func(error) render.Renderer {
	return func(err error) render.Renderer { return renderer(err, code) }
}

func onField(code, field string) func(error) render.Renderer {
	return func(err error) render.Renderer { return common.ErrorInvalidOnField(err, code, field) }
}
