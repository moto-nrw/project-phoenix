package timetracking

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The school's own wage types, surcharge rules and absence type mapping on
// /payroll (#4807). Behind config:manage like the fixed Lohnart settings:
// they describe how the payroll office books, no working time of a person.

var payrollRuleErrorRules = []common.ErrorRule{
	{Target: workforce.ErrPayrollRuleInvalid, Render: invalidWithCode(common.CodeWorkforcePayrollRuleInvalid)},
	{Target: workforce.ErrPayrollRuleNotFound, Render: notFoundWithCode(common.CodeWorkforcePayrollRuleNotFound)},
	{Target: workforce.ErrPayrollWageTypeNameTaken, Render: conflictWithCode(common.CodeWorkforcePayrollWageTypeNameTaken)},
	{Target: workforce.ErrPayrollWageTypeInUse, Render: conflictWithCode(common.CodeWorkforcePayrollWageTypeInUse)},
}

func renderPayrollRuleError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, common.RenderWithRules(err, payrollRuleErrorRules, common.ErrorInternalServer))
}

func (rs *StaffAdminResource) registerPayrollRuleRoutes(r chi.Router, withTx common.Middleware) {
	config := common.RequiresPermission(permissions.ConfigManage)
	r.With(config, withTx).Get("/time-tracking/payroll-rules", rs.getPayrollRules)
	r.With(config, withTx).Post("/time-tracking/payroll-rules/wage-types", rs.createPayrollWageType)
	r.With(config, withTx).Put("/time-tracking/payroll-rules/wage-types/{wageTypeId}", rs.updatePayrollWageType)
	r.With(config, withTx).Delete("/time-tracking/payroll-rules/wage-types/{wageTypeId}", rs.archivePayrollWageType)
	r.With(config, withTx).Post("/time-tracking/payroll-rules/surcharge-rules", rs.createPayrollSurchargeRule)
	r.With(config, withTx).Put("/time-tracking/payroll-rules/surcharge-rules/{ruleId}", rs.updatePayrollSurchargeRule)
	r.With(config, withTx).Delete("/time-tracking/payroll-rules/surcharge-rules/{ruleId}", rs.archivePayrollSurchargeRule)
	r.With(config, withTx).Put("/time-tracking/payroll-rules/absence-types/{absenceTypeId}", rs.setPayrollAbsenceTypeWageType)
}

// --- wire format ------------------------------------------------------------

// PayrollWageTypeResponse is one wage type; IDs travel as strings.
type PayrollWageTypeResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Number string `json:"number"`
	Unit   string `json:"unit"`
}

// PayrollSurchargeRuleResponse is one rule; the window is null for Sunday
// and holiday rules.
type PayrollSurchargeRuleResponse struct {
	ID                string `json:"id"`
	Kind              string `json:"kind"`
	WindowStartMinute *int   `json:"window_start_minute"`
	WindowEndMinute   *int   `json:"window_end_minute"`
	RateType          string `json:"rate_type"`
	RateHundredths    int    `json:"rate_hundredths"`
	WageTypeID        string `json:"wage_type_id"`
}

// PayrollAbsenceTypeResponse is one school absence type and its wage type.
type PayrollAbsenceTypeResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	IsActive   bool    `json:"is_active"`
	WageTypeID *string `json:"wage_type_id"`
}

// PayrollRulesResponse is the payroll rule block of /payroll.
type PayrollRulesResponse struct {
	WageTypes      []PayrollWageTypeResponse      `json:"wage_types"`
	SurchargeRules []PayrollSurchargeRuleResponse `json:"surcharge_rules"`
	AbsenceTypes   []PayrollAbsenceTypeResponse   `json:"absence_types"`
}

func idString(id int64) string { return strconv.FormatInt(id, 10) }

func toPayrollWageTypeResponse(value workforce.PayrollWageType) PayrollWageTypeResponse {
	return PayrollWageTypeResponse{ID: idString(value.ID), Name: value.Name, Number: value.Number, Unit: value.Unit}
}

func toPayrollSurchargeRuleResponse(value workforce.PayrollSurchargeRule) PayrollSurchargeRuleResponse {
	response := PayrollSurchargeRuleResponse{
		ID: idString(value.ID), Kind: value.Kind, RateType: value.RateType,
		RateHundredths: value.RateHundredths, WageTypeID: idString(value.WageTypeID),
	}
	if value.Kind == workforce.SurchargeKindNight {
		start, end := value.WindowStartMinute, value.WindowEndMinute
		response.WindowStartMinute, response.WindowEndMinute = &start, &end
	}
	return response
}

func toPayrollRulesResponse(overview *workforce.PayrollRulesOverview) PayrollRulesResponse {
	response := PayrollRulesResponse{
		WageTypes:      make([]PayrollWageTypeResponse, 0, len(overview.WageTypes)),
		SurchargeRules: make([]PayrollSurchargeRuleResponse, 0, len(overview.SurchargeRules)),
		AbsenceTypes:   make([]PayrollAbsenceTypeResponse, 0, len(overview.AbsenceTypes)),
	}
	for _, wageType := range overview.WageTypes {
		response.WageTypes = append(response.WageTypes, toPayrollWageTypeResponse(wageType))
	}
	for _, rule := range overview.SurchargeRules {
		response.SurchargeRules = append(response.SurchargeRules, toPayrollSurchargeRuleResponse(rule))
	}
	for _, absenceType := range overview.AbsenceTypes {
		entry := PayrollAbsenceTypeResponse{ID: idString(absenceType.AbsenceTypeID), Name: absenceType.Name, IsActive: absenceType.IsActive}
		if absenceType.WageTypeID != nil {
			id := idString(*absenceType.WageTypeID)
			entry.WageTypeID = &id
		}
		response.AbsenceTypes = append(response.AbsenceTypes, entry)
	}
	return response
}

// PayrollWageTypeRequest creates or changes a wage type.
type PayrollWageTypeRequest struct {
	workforce.PayrollWageTypeInput
}

func (p *PayrollWageTypeRequest) Bind(*http.Request) error { return nil }

// PayrollSurchargeRuleRequest creates or changes a rule. wage_type_id is the
// string ID of the wage type.
type PayrollSurchargeRuleRequest struct {
	Kind              string `json:"kind"`
	WindowStartMinute *int   `json:"window_start_minute"`
	WindowEndMinute   *int   `json:"window_end_minute"`
	RateType          string `json:"rate_type"`
	RateHundredths    int    `json:"rate_hundredths"`
	WageTypeID        string `json:"wage_type_id"`

	input workforce.PayrollSurchargeRuleInput
}

func (p *PayrollSurchargeRuleRequest) Bind(*http.Request) error {
	wageTypeID, err := strconv.ParseInt(p.WageTypeID, 10, 64)
	if err != nil || wageTypeID <= 0 {
		return errors.New("wage_type_id must be the ID of a wage type")
	}
	p.input = workforce.PayrollSurchargeRuleInput{
		Kind: p.Kind, WindowStartMinute: p.WindowStartMinute, WindowEndMinute: p.WindowEndMinute,
		RateType: p.RateType, RateHundredths: p.RateHundredths, WageTypeID: wageTypeID,
	}
	return nil
}

// PayrollAbsenceTypeRequest books an absence type onto a wage type; null
// books it with its category again.
type PayrollAbsenceTypeRequest struct {
	WageTypeID *string `json:"wage_type_id"`

	wageTypeID *int64
}

func (p *PayrollAbsenceTypeRequest) Bind(*http.Request) error {
	if p.WageTypeID == nil {
		return nil
	}
	id, err := strconv.ParseInt(*p.WageTypeID, 10, 64)
	if err != nil || id <= 0 {
		return errors.New("wage_type_id must be the ID of a wage type or null")
	}
	p.wageTypeID = &id
	return nil
}

// --- handlers ---------------------------------------------------------------

func (rs *StaffAdminResource) payrollActor(r *http.Request) int64 {
	return rs.identity(r.Context()).AccountID
}

func (rs *StaffAdminResource) getPayrollRules(w http.ResponseWriter, r *http.Request) {
	overview, err := rs.PayrollRules.Overview(r.Context())
	if err != nil {
		renderPayrollRuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toPayrollRulesResponse(overview), "Payroll rules retrieved")
}

func (rs *StaffAdminResource) createPayrollWageType(w http.ResponseWriter, r *http.Request) {
	req := &PayrollWageTypeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	created, err := rs.PayrollRules.CreateWageType(r.Context(), rs.payrollActor(r), req.PayrollWageTypeInput)
	if err != nil {
		renderPayrollRuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toPayrollWageTypeResponse(*created), "Wage type created")
}

func (rs *StaffAdminResource) updatePayrollWageType(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "wageTypeId", "invalid wage type ID")
	if !ok {
		return
	}
	req := &PayrollWageTypeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	updated, err := rs.PayrollRules.UpdateWageType(r.Context(), rs.payrollActor(r), id, req.PayrollWageTypeInput)
	if err != nil {
		renderPayrollRuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toPayrollWageTypeResponse(*updated), "Wage type updated")
}

func (rs *StaffAdminResource) archivePayrollWageType(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "wageTypeId", "invalid wage type ID")
	if !ok {
		return
	}
	if err := rs.PayrollRules.ArchiveWageType(r.Context(), rs.payrollActor(r), id); err != nil {
		renderPayrollRuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Wage type archived")
}

func (rs *StaffAdminResource) createPayrollSurchargeRule(w http.ResponseWriter, r *http.Request) {
	req := &PayrollSurchargeRuleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	created, err := rs.PayrollRules.CreateSurchargeRule(r.Context(), rs.payrollActor(r), req.input)
	if err != nil {
		renderPayrollRuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toPayrollSurchargeRuleResponse(*created), "Surcharge rule created")
}

func (rs *StaffAdminResource) updatePayrollSurchargeRule(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "ruleId", "invalid surcharge rule ID")
	if !ok {
		return
	}
	req := &PayrollSurchargeRuleRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	updated, err := rs.PayrollRules.UpdateSurchargeRule(r.Context(), rs.payrollActor(r), id, req.input)
	if err != nil {
		renderPayrollRuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toPayrollSurchargeRuleResponse(*updated), "Surcharge rule updated")
}

func (rs *StaffAdminResource) archivePayrollSurchargeRule(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "ruleId", "invalid surcharge rule ID")
	if !ok {
		return
	}
	if err := rs.PayrollRules.ArchiveSurchargeRule(r.Context(), rs.payrollActor(r), id); err != nil {
		renderPayrollRuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Surcharge rule archived")
}

func (rs *StaffAdminResource) setPayrollAbsenceTypeWageType(w http.ResponseWriter, r *http.Request) {
	id, ok := common.ParseInt64IDWithError(w, r, "absenceTypeId", "invalid absence type ID")
	if !ok {
		return
	}
	req := &PayrollAbsenceTypeRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.PayrollRules.SetAbsenceTypeWageType(r.Context(), rs.payrollActor(r), id, req.wageTypeID); err != nil {
		renderPayrollRuleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Absence type wage type updated")
}
