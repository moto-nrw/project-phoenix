package guardians

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	guardianSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Guardian payment endpoints (#2608). The whole section sits behind
// guardians:financial — deliberately NOT users:update: maintaining the
// guardian directory and handling bank data are different jobs.
//
// Every read is access-logged in the service; the handlers never touch an
// unmasked IBAN except in the reveal response and the export file.

// exportConfidentialityNote is stamped on every page of the exported list.
// The file leaves the building the moment it is downloaded, so the reminder
// belongs in the document, not only on the screen that produced it.
const (
	exportConfidentialityNote   = "Vertraulich. Enthält Bankverbindungen. Bitte nicht per E-Mail weitergeben."
	errInvalidPaymentGuardianID = "Die erziehungsberechtigte Person konnte nicht zugeordnet werden."
)

// GuardianPaymentMaskedResponse is the default bank read of one guardian.
type GuardianPaymentMaskedResponse struct {
	GuardianID    string  `json:"guardian_id"`
	IBANMasked    *string `json:"iban_masked"`
	AccountHolder *string `json:"account_holder"`
}

// GuardianPaymentRevealResponse carries the unmasked IBAN after the audited
// reveal.
type GuardianPaymentRevealResponse struct {
	GuardianID    string  `json:"guardian_id"`
	IBAN          *string `json:"iban"`
	AccountHolder *string `json:"account_holder"`
}

// GuardianPaymentRequest binds the bank PUT. An empty string clears a field.
type GuardianPaymentRequest struct {
	IBAN          *string `json:"iban"`
	AccountHolder *string `json:"account_holder"`
	Note          string  `json:"note"`
}

// Bind satisfies render.Binder; validation lives in the service.
func (r *GuardianPaymentRequest) Bind(_ *http.Request) error { return nil }

// StudentPayerRequest binds the per-child payer assignment. A nil
// guardian_id clears the assignment.
type StudentPayerRequest struct {
	GuardianID *string `json:"guardian_id"`
}

// Bind satisfies render.Binder.
func (r *StudentPayerRequest) Bind(_ *http.Request) error { return nil }

// PaymentOverviewRow is one line of the Bankverbindungen list.
type PaymentOverviewRow struct {
	StudentID        string  `json:"student_id"`
	StudentName      string  `json:"student_name"`
	SchoolClass      string  `json:"school_class"`
	GuardianID       *string `json:"guardian_id"`
	GuardianName     string  `json:"guardian_name"`
	RelationshipType string  `json:"relationship_type"`
	AccountHolder    string  `json:"account_holder"`
	IBANMasked       string  `json:"iban_masked"`
}

// PaymentExportRequest binds the export POST.
type PaymentExportRequest struct {
	Format listexport.Format `json:"format"`
}

// Bind satisfies render.Binder.
func (r *PaymentExportRequest) Bind(_ *http.Request) error { return nil }

// paymentActor pulls the acting account from the JWT for the GDPR access log.
// Role is the comma-joined role list, mirroring the staff financial shape.
func paymentActor(r *http.Request) (int64, string, error) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		return 0, "", errors.New("invalid token: no account id")
	}
	return int64(claims.ID), strings.Join(claims.Roles, ","), nil
}

// paymentErrorMessages renders each input error as the German sentence the
// school office reads in the toast. The service sentinels stay English for the
// code; only the wire message is translated, so a rule change is one line here
// rather than a string match in the frontend.
var paymentErrorMessages = []struct {
	sentinel error
	message  string
}{
	{guardianSvc.ErrGuardianIBANInvalid, "Die IBAN ist nicht gültig. Bitte prüfen Sie die Eingabe."},
	{guardianSvc.ErrGuardianAccountHolderTooLong, "Der Name des Kontoinhabers ist zu lang."},
	{guardianSvc.ErrGuardianNotLinkedToStudent, "Diese Person ist bei diesem Kind nicht als erziehungsberechtigt eingetragen."},
	{guardianSvc.ErrGuardianStudentRequired, "Das Kind konnte nicht zugeordnet werden."},
}

// renderPaymentError maps the service sentinels to their HTTP status and to a
// message the reader can act on.
func renderPaymentError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, userModels.ErrGuardianProfileNotFound) || errors.Is(err, guardianSvc.ErrStudentNotFound) {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}
	for _, rule := range paymentErrorMessages {
		if errors.Is(err, rule.sentinel) {
			//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(rule.message)))
			return
		}
	}
	if errors.Is(err, guardianSvc.ErrGuardianPaymentInvalid) {
		//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
		common.RenderError(w, r, common.ErrorInvalidRequest(
			errors.New("Die Eingabe ist nicht gültig. Bitte prüfen Sie die Felder.")))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}

func (rs *Resource) getGuardianPayment(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidPaymentGuardianID)))
		return
	}
	accountID, role, err := paymentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	masked, err := rs.GuardianService.GetGuardianPaymentMasked(r.Context(), id, accountID, role)
	if err != nil {
		renderPaymentError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, &GuardianPaymentMaskedResponse{
		GuardianID:    strconv.FormatInt(masked.GuardianProfileID, 10),
		IBANMasked:    masked.IBANMasked,
		AccountHolder: masked.AccountHolder,
	}, "Payment data retrieved successfully")
}

// revealGuardianPayment is a POST on purpose: serving the unmasked IBAN is an
// action that gets audited, not a cacheable read.
func (rs *Resource) revealGuardianPayment(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidPaymentGuardianID)))
		return
	}
	accountID, role, err := paymentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	plain, err := rs.GuardianService.RevealGuardianPayment(r.Context(), id, accountID, role)
	if err != nil {
		renderPaymentError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, &GuardianPaymentRevealResponse{
		GuardianID:    strconv.FormatInt(plain.GuardianProfileID, 10),
		IBAN:          plain.IBAN,
		AccountHolder: plain.AccountHolder,
	}, "Payment data revealed successfully")
}

func (rs *Resource) updateGuardianPayment(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidPaymentGuardianID)))
		return
	}
	req := &GuardianPaymentRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	accountID, _, err := paymentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	if err := rs.GuardianService.UpdateGuardianPayment(r.Context(), id, guardianSvc.GuardianPaymentInput{
		IBAN:          req.IBAN,
		AccountHolder: req.AccountHolder,
	}, accountID, req.Note); err != nil {
		renderPaymentError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Payment data updated successfully")
}

func (rs *Resource) setStudentPayer(w http.ResponseWriter, r *http.Request) {
	studentID, err := common.ParseIDParam(r, "studentId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return
	}
	req := &StudentPayerRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	accountID, _, err := paymentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	var guardianID *int64
	if req.GuardianID != nil && strings.TrimSpace(*req.GuardianID) != "" {
		parsed, perr := strconv.ParseInt(strings.TrimSpace(*req.GuardianID), 10, 64)
		if perr != nil || parsed <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errInvalidPaymentGuardianID)))
			return
		}
		guardianID = &parsed
	}

	if err := rs.GuardianService.SetStudentPayer(r.Context(), studentID, guardianID, accountID); err != nil {
		renderPaymentError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Payer updated successfully")
}

func (rs *Resource) listPaymentOverview(w http.ResponseWriter, r *http.Request) {
	accountID, role, err := paymentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	rows, err := rs.GuardianService.ListPaymentOverview(r.Context(), accountID, role)
	if err != nil {
		renderPaymentError(w, r, err)
		return
	}

	responses := make([]PaymentOverviewRow, 0, len(rows))
	for _, row := range rows {
		out := PaymentOverviewRow{
			StudentID:        strconv.FormatInt(row.StudentID, 10),
			StudentName:      row.StudentName,
			SchoolClass:      row.SchoolClass,
			GuardianName:     row.GuardianName,
			RelationshipType: row.RelationshipType,
			AccountHolder:    row.AccountHolder,
			IBANMasked:       row.IBANMasked,
		}
		if row.GuardianProfileID != nil {
			id := strconv.FormatInt(*row.GuardianProfileID, 10)
			out.GuardianID = &id
		}
		responses = append(responses, out)
	}

	common.Respond(w, r, http.StatusOK, responses, "Payment overview retrieved successfully")
}

func (rs *Resource) exportPaymentOverview(w http.ResponseWriter, r *http.Request) {
	if rs.ListExportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("list export service is not configured")))
		return
	}
	req := &PaymentExportRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	accountID, role, err := paymentActor(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	rows, err := rs.GuardianService.ListPaymentExportRows(r.Context(), accountID, role, string(req.Format))
	if err != nil {
		renderPaymentError(w, r, err)
		return
	}

	doc := listexport.Document{
		Title:       "Bankverbindungen",
		Subtitle:    paymentExportSubtitle(rows),
		GeneratedAt: time.Now(),
		Footer:      exportConfidentialityNote,
		Columns: []listexport.Column{
			{ID: listexport.ColumnStudentName, Label: "Kind"},
			{ID: listexport.ColumnStudentClass, Label: "Klasse"},
			{ID: listexport.ColumnContactName, Label: "Kontoinhaber"},
			{ID: listexport.ColumnIBAN, Label: "IBAN"},
		},
		Rows: buildPaymentExportRows(rows),
	}

	file, err := rs.ListExportService.Render(doc, req.Format, doc.Title)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

// paymentExportSubtitle states how complete the list is. A bank list that
// silently omits its gaps reads as finished when it is not.
func paymentExportSubtitle(rows []guardianSvc.GuardianPaymentRow) string {
	withIBAN := 0
	for _, row := range rows {
		if row.HasIBAN() {
			withIBAN++
		}
	}
	if withIBAN == len(rows) {
		return fmt.Sprintf("%d Kinder, alle mit Bankverbindung", len(rows))
	}
	return fmt.Sprintf("%d Kinder, davon %d mit Bankverbindung", len(rows), withIBAN)
}

// buildPaymentExportRows renders one line per child. Children without a payer
// or without an IBAN keep their row and say what is missing, so the list can
// be used as a to-do rather than read as complete.
func buildPaymentExportRows(rows []guardianSvc.GuardianPaymentRow) []listexport.Row {
	out := make([]listexport.Row, 0, len(rows))
	for _, row := range rows {
		holder := row.AccountHolder
		iban := row.IBAN
		switch {
		case row.GuardianProfileID == nil:
			holder = "Nicht zugeordnet"
			iban = "Fehlt"
		case iban == "":
			iban = "Fehlt"
		}
		out = append(out, listexport.Row{
			Values: map[listexport.ColumnID]string{
				listexport.ColumnStudentName:  row.StudentName,
				listexport.ColumnStudentClass: row.SchoolClass,
				listexport.ColumnContactName:  holder,
				listexport.ColumnIBAN:         iban,
			},
		})
	}
	return out
}
