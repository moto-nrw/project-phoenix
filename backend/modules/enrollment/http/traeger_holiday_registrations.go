package enrollmenthttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The review of the holiday registrations in the Träger-Portal (#4663, ADR
// 0114), mounted below /traeger/holiday-periods/{id}/registrations with the
// holiday periods: the same Träger session, the right "Anmeldung verwalten"
// and the Träger transaction of the session's Träger. The Träger and the
// person come from the session, never from the request. A confirmation
// freezes the contribution, so the Elternabrechnung serves it at
// /traeger/holiday-contributions (#4664, ADR 0115).

// registrationRoutes adds the review routes to the period routes.
func (rs *HolidayPeriodResource) registrationRoutes(r chi.Router, withTx common.Middleware) {
	review := r.With(rs.requireManage, withTx)
	review.Get("/{id}/registrations", rs.listRegistrations)
	r.With(rs.requireManage, rs.withSchoolNames, withTx).Get("/{id}/registrations/export", rs.exportRegistrations)
	review.Get("/{id}/registrations/{registrationId}", rs.getRegistration)
	review.Post("/{id}/registrations/{registrationId}/cancel", rs.registrationAction(capability.HolidayAuditCancel))
	review.Post("/{id}/registrations/{registrationId}/freeze", rs.registrationAction(capability.HolidayAuditFreeze))
	review.Post("/{id}/registrations/{registrationId}/reopen", rs.registrationAction(capability.HolidayAuditReopen))
	review.Put("/{id}/registrations/{registrationId}/children/{childId}/bookings", rs.rebookChild)
}

// reviewerOf returns the Träger and the account of the session, or renders
// 401.
func reviewerOf(w http.ResponseWriter, r *http.Request) (capability.HolidayReviewer, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 || principal.AccountID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return capability.HolidayReviewer{}, false
	}
	return capability.HolidayReviewer{OrganizationID: principal.OrganizationID(), AccountID: principal.AccountID()}, true
}

// registrationFilter reads status, school, week_id and time_model.
func registrationFilter(r *http.Request) (capability.HolidayRegistrationFilter, error) {
	query := r.URL.Query()
	filter := capability.HolidayRegistrationFilter{
		Status: strings.TrimSpace(query.Get("status")), School: strings.TrimSpace(query.Get("school")),
		TimeModel: strings.TrimSpace(query.Get("time_model")),
	}
	if raw := strings.TrimSpace(query.Get("week_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			return filter, fmt.Errorf("%w: week_id must be a positive ID", capability.ErrInvalidHolidayRegistrationFilter)
		}
		filter.WeekID = id
	}
	return filter, nil
}

func (rs *HolidayPeriodResource) listRegistrations(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	periodID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	filter, err := registrationFilter(r)
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	list, err := rs.review.ListHolidayRegistrations(r.Context(), reviewer, periodID, filter)
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, list, "Holiday registrations retrieved successfully")
}

func (rs *HolidayPeriodResource) getRegistration(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	detail, err := rs.review.HolidayRegistration(r.Context(), reviewer, periodID, registrationID)
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Holiday registration retrieved successfully")
}

// registrationAction answers cancel, freeze and reopen with the changed
// registration.
func (rs *HolidayPeriodResource) registrationAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reviewer, periodID, registrationID, ok := registrationKey(w, r)
		if !ok {
			return
		}
		detail, err := rs.review.ChangeHolidayRegistration(r.Context(), reviewer, periodID, registrationID, action)
		if err != nil {
			renderHolidayReviewError(w, r, err)
			return
		}
		common.Respond(w, r, http.StatusOK, detail, "Holiday registration changed successfully")
	}
}

// HolidayRebookingRequest is the body of PUT
// /traeger/holiday-periods/{id}/registrations/{registrationId}/children/{childId}/bookings:
// the new bookings of the child, one time model per week.
type HolidayRebookingRequest struct {
	Bookings []holidayBookingRequest `json:"bookings"`
}

// Bind accepts the body; the Owner validates the bookings.
func (req *HolidayRebookingRequest) Bind(*http.Request) error { return nil }

func (rs *HolidayPeriodResource) rebookChild(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	childID, err := common.ParseIDParam(r, "childId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	req := &HolidayRebookingRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	bookings := make([]capability.HolidayBookingInput, len(req.Bookings))
	for i, booking := range req.Bookings {
		bookings[i] = capability.HolidayBookingInput(booking)
	}
	detail, err := rs.review.RebookHolidayChild(r.Context(), reviewer, periodID, registrationID, childID, bookings)
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, detail, "Holiday registration rebooked successfully")
}

func registrationKey(w http.ResponseWriter, r *http.Request) (capability.HolidayReviewer, int64, int64, bool) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return capability.HolidayReviewer{}, 0, 0, false
	}
	periodID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return capability.HolidayReviewer{}, 0, 0, false
	}
	registrationID, err := common.ParseIDParam(r, "registrationId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return capability.HolidayReviewer{}, 0, 0, false
	}
	return reviewer, periodID, registrationID, true
}

// renderHolidayReviewError answers a refused review operation. Every refusal
// rolls the Träger transaction back: a write may already have run.
func renderHolidayReviewError(w http.ResponseWriter, r *http.Request, err error) {
	tenant.MarkRollback(r.Context())
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeEnrollmentHolidayRegistrationInvalid))
	case errors.Is(err, capability.ErrInvalidHolidayRegistrationFilter):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, capability.ErrHolidayPeriodNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayPeriodNotFound))
	case errors.Is(err, capability.ErrHolidayRegistrationEntryNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeEnrollmentHolidayRegistrationEntryNotFound))
	case errors.Is(err, capability.ErrHolidayRegistrationFrozen):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayRegistrationFrozen))
	case errors.Is(err, capability.ErrHolidayRegistrationStateConflict):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeEnrollmentHolidayRegistrationStateConflict))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

type holidaySchoolNamesKey struct{}

// withSchoolNames reads the school names of the session's Träger before the
// Träger transaction opens: the projection that knows them reads in its own
// administrative unit, which never nests in a Träger transaction.
func (rs *HolidayPeriodResource) withSchoolNames(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		organizationID, ok := traegerOf(w, r)
		if !ok {
			return
		}
		names, err := rs.schoolNames.HolidaySchoolNames(r.Context(), organizationID)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), holidaySchoolNamesKey{}, names)))
	})
}

// exportRegistrations answers GET .../registrations/export?format=csv|xlsx:
// one row per child and booked week of the filter.
func (rs *HolidayPeriodResource) exportRegistrations(w http.ResponseWriter, r *http.Request) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	periodID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	format := lists.Format(strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format"))))
	if format != lists.FormatCSV && format != lists.FormatXLSX {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("unsupported export format %q (use csv or xlsx)", format)))
		return
	}
	filter, err := registrationFilter(r)
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	export, err := rs.review.ExportHolidayRegistrations(r.Context(), reviewer, periodID, filter, string(format))
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	names, _ := r.Context().Value(holidaySchoolNamesKey{}).(map[int64]string)
	doc := holidayRegistrationDocument(export, filter, names)
	file, err := lists.NewRenderer().Render(doc, format, "ferienanmeldungen-"+export.Period.Name)
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

var holidayRegistrationStatusLabels = map[string]string{
	capability.HolidayRegistrationReceived:  "Eingegangen",
	capability.HolidayRegistrationConfirmed: "Bestätigt",
	capability.HolidayRegistrationCancelled: "Storniert",
}

// holidayRegistrationColumns are the fixed columns of the export. Health
// information stays out: the export is a list, not the record of a child.
// One column per question of the form without health information follows
// (#5198).
var holidayRegistrationColumns = []lists.Column{
	{ID: "status", Label: "Status"},
	{ID: "frozen", Label: "Eingefroren"},
	{ID: "registration", Label: "Anmeldung Nr."},
	{ID: "submitted_at", Label: "Eingegangen am"},
	{ID: "last_name", Label: "Nachname"},
	{ID: "first_name", Label: "Vorname"},
	{ID: "birth_date", Label: "Geburtsdatum"},
	{ID: "school", Label: "Schule"},
	{ID: "week", Label: "Woche"},
	{ID: "host_school", Label: "Ausrichtende OGS"},
	{ID: "time_model", Label: "Zeitmodell"},
	{ID: "guardian", Label: "Elternteil"},
	{ID: "email", Label: "E-Mail"},
	{ID: "phone", Label: "Telefon"},
	{ID: "second_guardian", Label: "Zweites Elternteil"},
	{ID: "second_guardian_phone", Label: "Telefon zweites Elternteil"},
	{ID: "duplicate", Label: "Mögliche Doppelanmeldung"},
}

func holidayRegistrationDocument(export capability.HolidayRegistrationExport, filter capability.HolidayRegistrationFilter, names map[int64]string) lists.Document {
	period, rows := export.Period, export.Rows
	schoolName := func(id int64) string {
		if name, ok := names[id]; ok {
			return name
		}
		return "Schule nicht mehr vorhanden"
	}
	out := make([]lists.Row, 0, len(rows))
	for _, row := range rows {
		school := "Feriengast"
		if row.Child.HomeSchoolID != nil {
			school = schoolName(*row.Child.HomeSchoolID)
		} else if row.Child.GuestSchoolName != "" {
			school = "Feriengast (" + row.Child.GuestSchoolName + ")"
		}
		duplicate := ""
		if len(row.Child.DuplicateRegistrationIDs) > 0 {
			numbers := make([]string, len(row.Child.DuplicateRegistrationIDs))
			for i, id := range row.Child.DuplicateRegistrationIDs {
				numbers[i] = strconv.FormatInt(id, 10)
			}
			duplicate = "Ja, Anmeldung Nr. " + strings.Join(numbers, ", ")
		}
		frozen := "Nein"
		if row.Registration.Frozen {
			frozen = "Ja"
		}
		secondName, secondPhone := holidaySecondGuardianContact(row.SecondGuardian)
		values := map[lists.ColumnID]string{
			"status": holidayRegistrationStatusLabels[row.Registration.Status], "frozen": frozen,
			"registration": strconv.FormatInt(row.Registration.ID, 10),
			"submitted_at": row.Registration.SubmittedAt.In(calendar.Berlin).Format("02.01.2006 15:04"),
			"last_name":    row.Child.LastName, "first_name": row.Child.FirstName,
			"birth_date": row.Child.BirthDate.Format("02.01.2006"), "school": school,
			"week":        row.Booking.WeekStartsOn.Format("02.01.") + " bis " + row.Booking.WeekEndsOn.Format("02.01.2006"),
			"host_school": schoolName(row.Booking.HostSchoolID), "time_model": row.Booking.TimeModelName,
			"guardian": row.Registration.GuardianFirstName + " " + row.Registration.GuardianLastName,
			"email":    row.Registration.GuardianEmail, "phone": row.Registration.GuardianPhone, "duplicate": duplicate,
			"second_guardian": secondName, "second_guardian_phone": secondPhone,
		}
		addHolidayAnswers(values, export.Columns, row.Answers)
		for id, value := range values {
			values[id] = lists.SanitizeUserText(value)
		}
		out = append(out, lists.Row{Values: values})
	}
	return lists.Document{
		Title: "Ferienanmeldungen", Subtitle: period.Name, GeneratedAt: time.Now(),
		Filters: holidayRegistrationFilterLabels(period, filter, schoolName), Columns: holidayExportColumns(export.Columns), Rows: out,
		Footer: "Vertraulich. Enthält personenbezogene Daten.",
	}
}

func holidayRegistrationFilterLabels(period capability.HolidayPeriod, filter capability.HolidayRegistrationFilter, schoolName func(int64) string) []string {
	labels := []string{}
	if filter.Status != "" {
		labels = append(labels, "Status: "+holidayRegistrationStatusLabels[filter.Status])
	}
	switch filter.School {
	case "":
	case capability.HolidayRegistrationGuestSchool:
		labels = append(labels, "Schule: Feriengäste")
	default:
		if id, err := strconv.ParseInt(filter.School, 10, 64); err == nil {
			labels = append(labels, "Schule: "+schoolName(id))
		}
	}
	for _, week := range period.Weeks {
		if week.ID == filter.WeekID {
			labels = append(labels, "Woche: "+week.StartsOn.Format("02.01.")+" bis "+week.EndsOn.Format("02.01.2006"))
		}
	}
	if filter.TimeModel != "" {
		labels = append(labels, "Zeitmodell: "+filter.TimeModel)
	}
	return labels
}

// holidaySecondGuardianContact keeps an absent optional card empty in both exports.
func holidaySecondGuardianContact(guardian *capability.HolidayGuardianInput) (string, string) {
	if guardian == nil {
		return "", ""
	}
	return guardian.FirstName + " " + guardian.LastName, guardian.Phone
}

// addHolidayAnswers adds the answers of a row to its question columns.
func addHolidayAnswers(values map[lists.ColumnID]string, columns []capability.HolidayFormField, answers capability.HolidayAnswers) {
	for _, column := range columns {
		if value, ok := answers[column.Key]; ok {
			values[holidayAnswerColumnID(column)] = holidayAnswerText(column, value)
		}
	}
}

// holidayExportColumns are the fixed columns and one column per question.
func holidayExportColumns(questions []capability.HolidayFormField) []lists.Column {
	columns := append([]lists.Column{}, holidayRegistrationColumns...)
	for _, question := range questions {
		columns = append(columns, lists.Column{ID: holidayAnswerColumnID(question), Label: lists.SanitizeUserText(question.Label)})
	}
	return columns
}

func holidayAnswerColumnID(question capability.HolidayFormField) lists.ColumnID {
	return lists.ColumnID("answer." + question.Key)
}

// holidayAnswerText writes an answer as the form shows it: Ja or Nein, the
// label of a choice, a German date.
func holidayAnswerText(question capability.HolidayFormField, raw json.RawMessage) string {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	switch answer := value.(type) {
	case bool:
		if answer {
			return "Ja"
		}
		return "Nein"
	case string:
		return holidayTextAnswer(question, answer)
	default:
		return fmt.Sprint(answer)
	}
}

// holidayTextAnswer names a choice by its label and writes a date in
// German.
func holidayTextAnswer(question capability.HolidayFormField, answer string) string {
	switch question.Type {
	case capability.FormFieldSelect:
		for _, option := range question.Options {
			if option.Value == answer {
				return option.Label
			}
		}
	case capability.FormFieldDate:
		if date, err := calendar.ParseDate(answer); err == nil {
			return date.Format("02.01.2006")
		}
	}
	return answer
}
