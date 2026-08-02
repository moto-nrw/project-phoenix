package staff

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Staff Stammdaten (#1423). Section-scoped sub-resources under
// /staff/{id}/stammdaten. The non-sensitive sections share the directory
// permission tier (users:read to see, users:update to edit); the bank & tax
// section sits exclusively behind staff:financial — deliberately NOT
// users:update, the directory maintainers are not the Träger payroll office.
// Every financial read is access-logged in the service; the handlers never
// touch unmasked values except in the reveal response.

// --- wire types ------------------------------------------------------------

// StammdatenPersonPayload is the person section on the wire.
type StammdatenPersonPayload struct {
	FirstName string         `json:"first_name"`
	LastName  string         `json:"last_name"`
	Birthday  *timezone.Date `json:"birthday"`
	Gender    *string        `json:"gender"`
}

// StammdatenKontaktPayload is the contact section on the wire.
type StammdatenKontaktPayload struct {
	AddressStreet         *string `json:"address_street"`
	AddressPostalCode     *string `json:"address_postal_code"`
	AddressCity           *string `json:"address_city"`
	Phone                 *string `json:"phone"`
	Email                 *string `json:"email"`
	EmergencyContactName  *string `json:"emergency_contact_name"`
	EmergencyContactPhone *string `json:"emergency_contact_phone"`
}

// StammdatenArbeitsvertragPayload is the contract section on the wire.
type StammdatenArbeitsvertragPayload struct {
	EntryDate        *timezone.Date `json:"entry_date"`
	ContractEndDate  *timezone.Date `json:"contract_end_date"`
	ProbationEndDate *timezone.Date `json:"probation_end_date"`
	WeeklyHours      *float64       `json:"weekly_hours"`
	EmploymentType   *string        `json:"employment_type"`
}

// StammdatenQualificationPayload is one qualification row on the wire.
type StammdatenQualificationPayload struct {
	ID         *int64         `json:"id,omitempty"`
	Name       string         `json:"name"`
	AcquiredOn *timezone.Date `json:"acquired_on"`
	ExpiresOn  *timezone.Date `json:"expires_on"`
}

// StammdatenResponse is the aggregate GET for the non-sensitive sections.
type StammdatenResponse struct {
	StaffID         int64                            `json:"staff_id"`
	Person          StammdatenPersonPayload          `json:"person"`
	Kontakt         StammdatenKontaktPayload         `json:"kontakt"`
	Arbeitsvertrag  StammdatenArbeitsvertragPayload  `json:"arbeitsvertrag"`
	Qualifikationen []StammdatenQualificationPayload `json:"qualifikationen"`
}

// StammdatenPersonRequest binds the person section PUT.
type StammdatenPersonRequest struct {
	StammdatenPersonPayload
	Note string `json:"note"`
}

func (r *StammdatenPersonRequest) Bind(_ *http.Request) error {
	if strings.TrimSpace(r.FirstName) == "" || strings.TrimSpace(r.LastName) == "" {
		return errors.New("first_name and last_name are required")
	}
	return nil
}

// StammdatenKontaktRequest binds the contact section PUT.
type StammdatenKontaktRequest struct {
	StammdatenKontaktPayload
	Note string `json:"note"`
}

func (r *StammdatenKontaktRequest) Bind(_ *http.Request) error { return nil }

// StammdatenArbeitsvertragRequest binds the contract section PUT.
type StammdatenArbeitsvertragRequest struct {
	StammdatenArbeitsvertragPayload
	Note string `json:"note"`
}

func (r *StammdatenArbeitsvertragRequest) Bind(_ *http.Request) error { return nil }

// StammdatenQualifikationenRequest binds the qualification list PUT (full
// replace).
type StammdatenQualifikationenRequest struct {
	Qualifikationen []StammdatenQualificationPayload `json:"qualifikationen"`
	Note            string                           `json:"note"`
}

func (r *StammdatenQualifikationenRequest) Bind(_ *http.Request) error {
	for _, q := range r.Qualifikationen {
		if strings.TrimSpace(q.Name) == "" {
			return errors.New("qualification name is required")
		}
	}
	return nil
}

// StammdatenFinancialRequest binds the bank & tax PUT.
type StammdatenFinancialRequest struct {
	IBAN                 *string `json:"iban"`
	TaxID                *string `json:"tax_id"`
	SocialSecurityNumber *string `json:"social_security_number"`
	Note                 string  `json:"note"`
}

func (r *StammdatenFinancialRequest) Bind(_ *http.Request) error { return nil }

// StammdatenFinancialMaskedResponse is the default financial read.
type StammdatenFinancialMaskedResponse struct {
	StaffID                    int64   `json:"staff_id"`
	IBANMasked                 *string `json:"iban_masked"`
	TaxIDMasked                *string `json:"tax_id_masked"`
	SocialSecurityNumberMasked *string `json:"social_security_number_masked"`
}

// StammdatenFinancialRevealResponse carries the unmasked values after the
// audited reveal.
type StammdatenFinancialRevealResponse struct {
	StaffID              int64   `json:"staff_id"`
	IBAN                 *string `json:"iban"`
	TaxID                *string `json:"tax_id"`
	SocialSecurityNumber *string `json:"social_security_number"`
}

// --- handlers --------------------------------------------------------------

// getStammdaten serves the non-sensitive sections in one response — they
// share one read permission, so separate GETs would only multiply requests.
func (rs *Resource) getStammdaten(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	data, err := rs.PersonService.GetStaffStammdaten(r.Context(), id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, newStammdatenResponse(data), "Stammdaten retrieved successfully")
}

func newStammdatenResponse(data *usersSvc.StaffStammdaten) *StammdatenResponse {
	resp := &StammdatenResponse{
		StaffID:         data.Staff.ID,
		Qualifikationen: make([]StammdatenQualificationPayload, 0, len(data.Qualifications)),
	}
	if person := data.Staff.Person; person != nil {
		resp.Person.FirstName = person.FirstName
		resp.Person.LastName = person.LastName
		resp.Person.Birthday = person.Birthday
	}
	resp.Arbeitsvertrag.EmploymentType = data.Staff.EmploymentType
	if md := data.MasterData; md != nil {
		resp.Person.Gender = md.Gender
		resp.Kontakt.AddressStreet = md.AddressStreet
		resp.Kontakt.AddressPostalCode = md.AddressPostalCode
		resp.Kontakt.AddressCity = md.AddressCity
		resp.Kontakt.Phone = md.Phone
		resp.Kontakt.Email = md.Email
		resp.Kontakt.EmergencyContactName = md.EmergencyContactName
		resp.Kontakt.EmergencyContactPhone = md.EmergencyContactPhone
		resp.Arbeitsvertrag.EntryDate = md.EntryDate
		resp.Arbeitsvertrag.ContractEndDate = md.ContractEndDate
		resp.Arbeitsvertrag.ProbationEndDate = md.ProbationEndDate
		resp.Arbeitsvertrag.WeeklyHours = md.WeeklyHours
	}
	for _, q := range data.Qualifications {
		qualID := q.ID
		resp.Qualifikationen = append(resp.Qualifikationen, StammdatenQualificationPayload{
			ID:         &qualID,
			Name:       q.Name,
			AcquiredOn: q.AcquiredOn,
			ExpiresOn:  q.ExpiresOn,
		})
	}
	return resp
}

// stammdatenAck is the uniform PUT response; the tab refetches the section
// afterwards, so the ack carries no data.
type stammdatenAck struct {
	StaffID int64 `json:"staff_id"`
}

// runStammdatenUpdate is the shared PUT shape for non-financial sections:
// parse id, resolve the acting staff member, run the section update, classify
// errors. Financial writers can be external payroll accounts without a staff
// mapping, so their endpoint resolves its account actor separately.
func (rs *Resource) runStammdatenUpdate(w http.ResponseWriter, r *http.Request, update func(staffID, changedBy int64) error) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	changedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	if err := update(id, changedBy); err != nil {
		switch {
		case errors.Is(err, usersSvc.ErrStaffStammdatenInvalid):
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "stammdaten_invalid"))
		case modelBase.IsNoRows(err):
			common.RenderError(w, r, common.ErrorNotFound(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.Respond(w, r, http.StatusOK, &stammdatenAck{StaffID: id}, "Stammdaten updated successfully")
}

func (rs *Resource) updateStammdatenPerson(w http.ResponseWriter, r *http.Request) {
	req := &StammdatenPersonRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rs.runStammdatenUpdate(w, r, func(staffID, changedBy int64) error {
		return rs.PersonService.UpdateStaffStammdatenPerson(r.Context(), staffID, usersSvc.StammdatenPersonInput{
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Birthday:  req.Birthday,
			Gender:    req.Gender,
		}, changedBy, req.Note)
	})
}

func (rs *Resource) updateStammdatenKontakt(w http.ResponseWriter, r *http.Request) {
	req := &StammdatenKontaktRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rs.runStammdatenUpdate(w, r, func(staffID, changedBy int64) error {
		return rs.PersonService.UpdateStaffStammdatenKontakt(r.Context(), staffID, usersSvc.StammdatenKontaktInput{
			AddressStreet:         req.AddressStreet,
			AddressPostalCode:     req.AddressPostalCode,
			AddressCity:           req.AddressCity,
			Phone:                 req.Phone,
			Email:                 req.Email,
			EmergencyContactName:  req.EmergencyContactName,
			EmergencyContactPhone: req.EmergencyContactPhone,
		}, changedBy, req.Note)
	})
}

func (rs *Resource) updateStammdatenArbeitsvertrag(w http.ResponseWriter, r *http.Request) {
	req := &StammdatenArbeitsvertragRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rs.runStammdatenUpdate(w, r, func(staffID, changedBy int64) error {
		return rs.PersonService.UpdateStaffStammdatenArbeitsvertrag(r.Context(), staffID, usersSvc.StammdatenArbeitsvertragInput{
			EntryDate:        req.EntryDate,
			ContractEndDate:  req.ContractEndDate,
			ProbationEndDate: req.ProbationEndDate,
			WeeklyHours:      req.WeeklyHours,
			EmploymentType:   req.EmploymentType,
		}, changedBy, req.Note)
	})
}

func (rs *Resource) updateStammdatenQualifikationen(w http.ResponseWriter, r *http.Request) {
	req := &StammdatenQualifikationenRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rs.runStammdatenUpdate(w, r, func(staffID, changedBy int64) error {
		inputs := make([]usersSvc.StammdatenQualificationInput, 0, len(req.Qualifikationen))
		for _, q := range req.Qualifikationen {
			inputs = append(inputs, usersSvc.StammdatenQualificationInput{
				Name:       q.Name,
				AcquiredOn: q.AcquiredOn,
				ExpiresOn:  q.ExpiresOn,
			})
		}
		return rs.PersonService.ReplaceStaffQualifications(r.Context(), staffID, inputs, changedBy, req.Note)
	})
}

// financialActor pulls the acting account from the JWT for the GDPR access
// log. Role is the comma-joined role list, mirroring the attendance-history
// audit shape.
func financialActor(r *http.Request) (int64, string, error) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return 0, "", errors.New("invalid token: no account id")
	}
	return int64(claims.ID), strings.Join(claims.Roles, ","), nil
}

func (rs *Resource) getStammdatenFinancial(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	accountID, role, err := financialActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	masked, err := rs.PersonService.GetStaffFinancialMasked(r.Context(), id, accountID, role)
	if err != nil {
		if modelBase.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, &StammdatenFinancialMaskedResponse{
		StaffID:                    masked.StaffID,
		IBANMasked:                 masked.IBANMasked,
		TaxIDMasked:                masked.TaxIDMasked,
		SocialSecurityNumberMasked: masked.SocialSecurityNumberMasked,
	}, "Financial data retrieved successfully")
}

// revealStammdatenFinancial is a POST on purpose: serving the unmasked
// values writes an audit row, so the read has a side effect.
func (rs *Resource) revealStammdatenFinancial(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	accountID, role, err := financialActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	plain, err := rs.PersonService.RevealStaffFinancial(r.Context(), id, accountID, role)
	if err != nil {
		if modelBase.IsNoRows(err) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, &StammdatenFinancialRevealResponse{
		StaffID:              plain.StaffID,
		IBAN:                 plain.IBAN,
		TaxID:                plain.TaxID,
		SocialSecurityNumber: plain.SocialSecurityNumber,
	}, "Financial data revealed successfully")
}

func (rs *Resource) updateStammdatenFinancial(w http.ResponseWriter, r *http.Request) {
	req := &StammdatenFinancialRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	staffID, ok := common.ParseInt64IDWithError(w, r, "id", common.MsgInvalidStaffID)
	if !ok {
		return
	}
	accountID, _, err := financialActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	if err := rs.PersonService.UpdateStaffFinancial(r.Context(), staffID, usersSvc.StammdatenFinancialInput{
		IBAN:                 req.IBAN,
		TaxID:                req.TaxID,
		SocialSecurityNumber: req.SocialSecurityNumber,
	}, accountID, req.Note); err != nil {
		switch {
		case errors.Is(err, usersSvc.ErrStaffStammdatenInvalid):
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "stammdaten_invalid"))
		case modelBase.IsNoRows(err):
			common.RenderError(w, r, common.ErrorNotFound(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.Respond(w, r, http.StatusOK, &stammdatenAck{StaffID: staffID}, "Stammdaten updated successfully")
}
